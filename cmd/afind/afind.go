package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/andaru/afind"
	"github.com/op/go-logging"
)

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
	flagKeys afind.FlagStringSlice
	flagMeta = make(afind.FlagSSMap)

	// Index flagset
	flagSetIndex = flag.NewFlagSet("index", flag.ExitOnError)

	// Repos flagset
	flagSetRepos   = flag.NewFlagSet("repos", flag.ExitOnError)
	flagRepoDelete = flagSetRepos.Bool("D", false,
		"Delete a single repo if selected")

	log = logging.MustGetLogger("afind")
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
	remotes  afind.Remotes
}

func newContext() (*ctx, error) {
	keys := make([]string, len(flagKeys))
	for i, k := range flagKeys {
		keys[i] = k
	}
	remotes := afind.NewRemotes()
	remotes.RegisterAddress(*flagRpcAddress)
	c := &ctx{
		repoKeys: keys,
		meta:     flagMeta,
		remotes:  remotes,
	}
	return c, nil
}

func search(context *ctx, query string) error {
	request := afind.SearchRequest{
		Re:            query,
		PathRe:        *flagSearchPath,
		CaseSensitive: !*flagSearchInsens,
		RepoKeys:      flagKeys,
		Meta:          flagMeta,
	}

	client, err := context.remotes.Get(*flagRpcAddress)
	if err != nil {
		return err
	}
	if client == nil {
		fmt.Fprintf(os.Stderr, "error contacting server '%s'\n",
			*flagRpcAddress)
		return err
	}
	request.SetRecursion(true)
	sr, err := client.Search(request)
	if err == nil {
		printMatches(sr)
	}
	return err
}

func index(context *ctx, key, root string, subdirs []string) error {
	request := afind.IndexRequest{
		Key:  key,
		Root: root,
		Dirs: subdirs,
		Meta: flagMeta,
	}
	client, err := context.remotes.Get(*flagRpcAddress)
	if err != nil {
		return err
	}
	if client == nil {
		fmt.Fprintf(os.Stderr, "error contacting server '%s'\n",
			*flagRpcAddress)
		return err
	}

	ir, err := client.Index(request)
	if err == nil {
		fmt.Printf("indexed [%v] meta: %v in %v\n",
			ir.Repo.Key, ir.Repo.Meta, ir.Elapsed)
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

	return (fmt.Sprintf("repo key: %s (state: %s)\n", r.Key, r.State) +
		fmt.Sprintf("  root path  : %v\n", r.Root) +
		fmt.Sprintf("  data size  : %s\n", afind.ByteSize(r.SizeData)) +
		fmt.Sprintf("  index size : %s\n", afind.ByteSize(r.SizeIndex)) +
		fmt.Sprintf("  total files: %d\n", r.NumFiles) +
		fmt.Sprintf("  total dirs : %d\n", r.NumDirs) +
		fmt.Sprintf("  metadata   : %v\n", meta))
}

func repos(context *ctx, key string) error {
	var err error

	client, err := context.remotes.Get(*flagRpcAddress)
	if err != nil {
		return err
	}
	if client == nil {
		fmt.Fprintf(os.Stderr, "error contacting server '%s'\n",
			*flagRpcAddress)
		return err
	}

	if key == "" {
		// get all repos
		rs, allerr := client.GetAllRepos()
		if allerr != nil {
			return allerr
		}
		for _, v := range *rs {
			fmt.Println(repoAsString(v))
		}
	} else {
		// or just one by argument
		if !*flagRepoDelete {
			repos, err := client.GetRepo(key)
			if err == nil {
				for _, repo := range *repos {
					fmt.Println(repoAsString(repo))
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "deleting repo %s", key)
			err = client.DeleteRepo(key)
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
		// args, minimum 1
		// <repo key (prefix)> <repo root path> [subdir..]
		if len(args) < 2 {
			// usage
			flagSetIndex.Usage()
			return nil
		}
		key := args[0]
		root := args[1]
		subdirs := args[2:]
		err = index(context, key, root, subdirs)
		if err != nil {
			return err
		}
	case "search":
		err = flagSetSearch.Parse(args)
		if err != nil {
			flagSetSearch.Usage()
			return err
		}
		if len(args) == 1 {
			err = search(context, strings.Join(args, " "))
		} else {
			flagSetSearch.Usage()
		}
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

func printMatches(sr *afind.SearchResponse) {
	for name, repos := range sr.Files {
		for repo, matches := range repos {
			for l, text := range matches {
				fmt.Printf("%s %s:%s:%s",
					repo, name, l, text)
			}
		}
	}
}

func main() {
	flag.Parse()
	afind.LoggerStderr()
	logging.SetFormatter(logging.DefaultFormatter)

	if len(flag.Args()) < 1 {
		flag.Usage()
		return
	}
	err := doAfind()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
}
