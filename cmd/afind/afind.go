package main

import (
	"errors"
	"flag"
	"fmt"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"strings"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/afind/api"
	"github.com/andaru/afind/flags"
)

// This is the afind command-line client

var (
	// Master flagset options
	// Afind master to connect to
	flagRpcAddress = flag.String("server", ":30800",
		"Afind server RPC address")

	// Search flagset options (for afind search -opt)
	flagSetSearch = flag.NewFlagSet("search", flag.ExitOnError)

	// -f "src/foo/bar" : only match files whose name matches this regexp
	flagSearchPath = flagSetSearch.String("f", "",
		"Search only in file names matching this regexp")
	flagSearchInsens = flagSetSearch.Bool("i", false,
		"Case insensitive search")

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
  afind <command> [-options] <operation arguments...>

Available commands (try 'afind <command> -h'):

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
  afind index [-D k1=v1,k2=v2,...] <key> <root> <dirN> <dirN..>

Where:
  key     Unique key for this Repo
  root    The absolute path to the root of the Repo
  dirN    One or more sub directories of root.
          To index all of root, use the single dir '.'

Options:`)
	flagSetIndex.PrintDefaults()
}

func usageRepos() {
	fmt.Fprintln(os.Stderr, `afind repos : display and delete repositories

Usage:
  afind repos [-D] [key] [key...]

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
	flag.Var(&flagKeys, "key",
		"Search just this comma-separated list of repository keys")
	flagSetIndex.Var(&flagMeta, "D",
		"A key=value pair to add to index or search request metadata")
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

func newContext() (*ctx, error) {
	cl, err := rpc.Dial("tcp", *flagRpcAddress)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(flagKeys))
	for i, k := range flagKeys {
		keys[i] = k
	}
	context := &ctx{
		repoKeys: keys,
		meta:     flagMeta,
		client:   cl,
	}
	context.repos = api.NewReposClient(cl)
	context.searcher = api.NewSearcherClient(cl)
	context.indexer = api.NewIndexerClient(cl)
	return context, nil
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
		// Timeout:    time.Duration(*flagTimeoutSearch) * time.Second,
	}
	request.Recurse = true
	request.Context = getSearchContext()
	sr, err := c.searcher.Search(context.Background(), request)
	if err == nil && sr.Error == "" {
		printMatches(sr)
	}
	if sr.Error != "" {
		err = errors.New(sr.Error)
	}
	return err
}

func index(c *ctx, key, root string, subdirs []string) error {
	request := afind.IndexQuery{
		Key:     key,
		Root:    root,
		Dirs:    subdirs,
		Meta:    afind.Meta(flagMeta),
		Recurse: true,
	}
	ir, err := c.indexer.Index(context.Background(), request)
	if ir.Repo != nil {
		fmt.Printf("index [%s] done in %v\n",
			ir.Repo.Key, ir.Repo.ElapsedIndexing)
	}
	if ir.Error != "" {
		err = errors.New(ir.Error)
	}
	return err
}

func repoAsString(r *afind.Repo) string {
	// convert the metadata to a json object style
	meta := "{"
	for k, v := range r.Meta {
		meta += k + `:` + ` "` + v + `", `
	}
	meta = meta[:len(meta)-2] + "}"

	return (fmt.Sprintf("repo: %s [%s]\n", r.Key, r.State) +
		fmt.Sprintf("  indexed:    %v (in %v)\n",
			r.TimeCreated, r.ElapsedIndexing) +
		fmt.Sprintf("  state:      %s\n", r.State) +
		fmt.Sprintf("  root path:  %v\n", r.Root) +
		fmt.Sprintf("  data size:  %s\n", r.SizeData) +
		fmt.Sprintf("  index size: %s\n", r.SizeIndex) +
		fmt.Sprintf("  files/dirs: %d/%d\n", r.NumFiles, r.NumDirs) +
		fmt.Sprintf("  metadata:   %v\n", meta))
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
	context, err := newContext()
	if err != nil {
		return err
	}

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
		subdirs := args[2:]
		return index(context, key, root, subdirs)
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
		return search(context, strings.Join(args, " "))
	case "repos":
		err = flagSetRepos.Parse(args)
		if err != nil {
			flagSetRepos.Usage()
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
