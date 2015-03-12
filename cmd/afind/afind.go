package main

import (
	"errors"
	"flag"
	"fmt"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/afind/api"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/flags"
	"github.com/andaru/afind/utils"
)

// This is the afind command-line client

var (
	// Master flagset options
	// Afind master to connect to
	flagRpcAddress = flag.String("server", "", "Afind server RPC address")
	// Verbose output mode
	flagVerbose = flag.Bool("v", false, "Verbose output")

	// Search flagset options (for afind search -opt)
	flagSetSearch = flag.NewFlagSet("search", flag.ExitOnError)

	// -f "src/foo/bar" : only match files whose name matches this regexp
	flagSearchPath = flagSetSearch.String("f", "",
		"Search only in file names matching this regexp")
	flagSearchInsens = flagSetSearch.Bool("i", false,
		"Case insensitive search")
	flagMaxMatches = flagSetSearch.Uint64("n", 100, "Limit results to NUM matches")

	// -key 1,2 -key 3 : one or more comma separated groups of keys
	flagKeys flags.StringSlice
	flagMeta = make(flags.SSMap)

	// context, -An, -Bn, -Cn
	flagContextPost = flagSetSearch.Int("A", 0, "Print NUM lines of trailing context")
	flagContextPre  = flagSetSearch.Int("B", 0, "Print NUM lines of leading context")
	flagContextBoth = flagSetSearch.Int("C", 0, "Print NUM lines of output context")

	// Index flagset
	flagSetIndex = flag.NewFlagSet("index", flag.ExitOnError)

	// Repos flagset
	flagSetRepos   = flag.NewFlagSet("repos", flag.ExitOnError)
	flagRepoDelete = flagSetRepos.Bool("D", false,
		"Delete a single repo if selected")
	flagTimeoutSearch = flag.Float64("timeout", 30.0,
		"Set the search timeout in seconds")
)

func usage() {
	fmt.Println(`afind : distributed text search

Usage:
  afind [global options] <command> [command options] <arguments..>

Available commands (for command options, see 'afind <command> -h'):

  index      Index text, creating a Repo (repository)
  search     Search for text in one more or all Repo
  repos      Display details of one or all Repo

Global options:
`)
	flag.PrintDefaults()
}

func usageIndex() {
	fmt.Fprintln(os.Stderr, `afind index : create repositories of indexed text

Usage:
  afind index [options] <key> <root> <dirN> [dirN..]

Where:
  key     Unique key for this Repo
  root    The absolute path to the root of the Repo
  dirN    One or more sub directories of root.
          To index everything under root, just use '.'

Options:`)
	flagSetIndex.PrintDefaults()
}

func usageRepos() {
	fmt.Fprintln(os.Stderr, `afind repos : display and delete repositories

Usage:
  afind repos [-D] [key] [key..]

If a single key only is provided, -D will delete that repository.
Otherwise, details about the one repository are displayed.  If key is
not provided, -D is not available and details of all repositories are
printed.

Options:`)
	flagSetRepos.PrintDefaults()
}

func usageSearch() {
	fmt.Fprintln(os.Stderr, `afind search : search repositories for text

Usage:
  afind search [-i] [-f pathre] <regular expression>

Examples:
  Search for 'this thing' or 'that thing':
  $ afind search -i "(this thing|that thing)"

Options:`)
	flagSetSearch.PrintDefaults()
}

func init() {
	flagSetSearch.Var(&flagKeys, "key",
		"Search just this comma-separated list of repository keys")
	flagSetSearch.Var(&flagMeta, "m",
		"A key value pair found in Repo to search")
	flagSetIndex.Var(&flagMeta, "m",
		"A key value pair added to merge with query Repo metadata when indexed")
	flag.Usage = usage
	flagSetSearch.Usage = usageSearch
	flagSetIndex.Usage = usageIndex
	flagSetRepos.Usage = usageRepos
}

// the union context for any single command execution
type ctx struct {
	repoKeys []string
	meta     map[string]string

	client   *rpc.Client
	repos    *api.ReposClient
	searcher *api.SearcherClient
	indexer  *api.IndexerClient
}

// Gets the server address from the environment and the --server
// flag. if the flag is provided, it overrides the AFIND_SERVER
// environment variable. The default RPC port number will be
// appended if it is not included in the source value.
func getFlagRpcAddress() string {
	if *flagRpcAddress != "" {
		return *flagRpcAddress
	} else {
		server := os.Getenv("AFIND_SERVER")
		if server == "" {
			return ":" + utils.DefaultRpcPort
		} else if !strings.Contains(server, ":") {
			return ":" + server
		} else {
			return server
		}
	}
}

func setupContext(context *ctx) error {
	cl, err := rpc.Dial("tcp", getFlagRpcAddress())
	if err != nil {
		return err
	}
	context.repos = api.NewReposClient(cl)
	context.searcher = api.NewSearcherClient(cl)
	context.indexer = api.NewIndexerClient(cl)
	return nil
}

func newContext() *ctx {
	keys := make([]string, len(flagKeys))
	for i, k := range flagKeys {
		keys[i] = k
	}
	context := &ctx{
		repoKeys: keys,
		meta:     flagMeta,
	}
	return context
}

func getSearchContext() afind.SearchContext {
	return afind.SearchContext{
		Both: *flagContextBoth,
		Pre:  *flagContextPre,
		Post: *flagContextPost,
	}
}

func search(c *ctx, query string) error {
	request := afind.SearchQuery{
		Re:         query,
		PathRe:     *flagSearchPath,
		IgnoreCase: *flagSearchInsens,
		RepoKeys:   flagKeys,
		Meta:       afind.Meta(flagMeta),
		MaxMatches: *flagMaxMatches,
		Recurse:    true,
		// Timeout:    time.Duration(*flagTimeoutSearch) * time.Second,
	}
	request.Context = getSearchContext()
	sr, err := c.searcher.Search(context.Background(), request)
	fmt.Printf("sr is: %#v\n", sr)
	fmt.Printf("err is: %#v\n", err)
	// now print the matches
	printMatches(sr)
	// print per repo errors, if any were found
	printErrors(sr)
	if *flagVerbose {
		// print repo information
		printRepos(sr)
	}

	if sr.Error != "" {
		err = errors.New(sr.Error)
	}
	return err
}

func index(c *ctx, key, root string, dirsOrFiles []string) error {
	request := afind.IndexQuery{
		Key:     key,
		Root:    root,
		Dirs:    []string{},
		Files:   []string{},
		Meta:    afind.Meta(flagMeta),
		Recurse: true,
	}
	// Scan the dirsOrFiles to see which are which, and add them
	// appropriately to the request
	for _, path := range dirsOrFiles {
		fullpath := filepath.Join(root, path)
		if fi, err := os.Lstat(fullpath); err != nil {
			if fi.IsDir() {
				request.Dirs = append(request.Dirs, path)
			} else {
				request.Files = append(request.Files, path)
			}
		}
	}

	ir, err := c.indexer.Index(context.Background(), request)
	if ir.Repo != nil {
		fmt.Printf("index [%s] done in %v\n",
			ir.Repo.Key, ir.Repo.ElapsedIndexing)
	} else if err != nil {
		return err
	}
	// Compare the error value to nil in its type, return the
	// error if so.
	if ir.Error != (*errs.StructError)(nil) {
		return ir.Error
	}
	return nil
}

func repoAsString(r *afind.Repo) string {
	// convert the metadata to a json object style
	meta := "{}"
	if len(r.Meta) > 0 {
		meta = "{"
		for k, v := range r.Meta {
			meta += k + `:` + ` "` + v + `", `
		}
		meta = meta[:len(meta)-2] + "}"
	}

	return (fmt.Sprintf("repo: %s [%s]\n", r.Key, r.State) +
		fmt.Sprintf("  last updated: %v\n", r.TimeUpdated) +
		fmt.Sprintf("  indexed in:   %v\n", r.ElapsedIndexing) +
		fmt.Sprintf("  state:        %s\n", r.State) +
		fmt.Sprintf("  root path:    %v\n", r.Root) +
		fmt.Sprintf("  data size:    %s\n", r.SizeData) +
		fmt.Sprintf("  index size:   %s\n", r.SizeIndex) +
		fmt.Sprintf("  files:        %d\n", r.NumFiles) +
		fmt.Sprintf("  metadata:     %v\n", meta))
}

func repos(c *ctx, key string) error {
	var err error

	if key == "" {
		// get all repos
		repos, allerr := c.repos.GetAll()
		if allerr != nil {
			return allerr
		}
		for _, v := range repos {
			fmt.Println(repoAsString(v))
		}
	} else {
		// or just one by argument
		if !*flagRepoDelete {
			repos, err := c.repos.Get(key)
			if err == nil {
				for _, repo := range repos {
					fmt.Println(repoAsString(repo))
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "deleting repo %s", key)
			err = c.repos.Delete(key)
		}
	}
	return err
}

func doAfind() error {
	var err error

	command := strings.ToLower(flag.Arg(0))
	args := flag.Args()[1:]
	context := newContext()

	switch command {
	case "index":
		err = flagSetIndex.Parse(args)
		if err != nil {
			flagSetIndex.Usage()
			return err
		}
		args := flagSetIndex.Args()
		// arguments, must have at least 1 subdir
		// <key> <rootdir> <subdir> [subdir...]
		if len(args) < 3 {
			// usage
			flagSetIndex.Usage()
			return nil
		}
		key := args[0]
		root := args[1]
		dirsOrFiles := args[2:]
		if err = setupContext(context); err == nil {
			return index(context, key, root, dirsOrFiles)
		}
	case "search":
		err = flagSetSearch.Parse(args)
		if err != nil {
			flagSetSearch.Usage()
			return err
		}
		args := flagSetSearch.Args()
		if len(args) < 1 {
			// usage
			flagSetSearch.Usage()
			return nil
		}
		if err = setupContext(context); err == nil {
			return search(context, strings.Join(args, " "))
		}
	case "repos":
		err = flagSetRepos.Parse(args)
		if err != nil {
			flagSetRepos.Usage()
			return err
		}
		if err = setupContext(context); err != nil {
			return err
		}
		if len(args) == 0 {
			err = repos(context, "")
		} else if len(args) > 0 {
			for _, arg := range args {
				err = repos(context, arg)
			}
		}
	}
	return err
}

func printMatches(sr *afind.SearchResult) {
	for name, repos := range sr.Matches {
		for repo, matches := range repos {
			nums := make([]int, 0)
			for l, _ := range matches {
				if linenum, err := strconv.Atoi(l); err == nil {
					nums = append(nums, linenum)
				}
			}
			sort.Ints(nums)
			for _, linenum := range nums {
				textlinenum := strconv.Itoa(linenum)
				fmt.Printf("%s:%s:%s:%s",
					repo, name, textlinenum, matches[textlinenum])
			}
		}
	}
}

func printErrors(sr *afind.SearchResult) {
	first := true
	pfirst := func() {
		if first {
			fmt.Println("Errors:")
			first = false
		}
	}
	for key, err := range sr.Errors {
		if err.T != "" {
			pfirst()
			fmt.Printf("repo %s [%s]", key, err.Type())
			if err.Message() != "" {
				fmt.Printf(" %s", err.Message())
			}
			fmt.Println("")
		}
	}
	if sr.Error != "" {
		pfirst()
		fmt.Println(sr.Error)
	}
}

func printRepos(sr *afind.SearchResult) {
	fmt.Println("Repos in result:")
	for _, repo := range sr.Repos {
		fmt.Println(repoAsString(repo))
	}
}

func main() {
	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(2)
		return
	}
	err := doAfind()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
