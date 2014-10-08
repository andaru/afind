package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/andaru/afind"
	"github.com/andaru/afind/common"
	"github.com/op/go-logging"
)

var (
	// Afind master to connect to
	flagRpcAddress = flag.String("server", ":30800",
		"Afind master RPC server address")
	// -f "src/foo/bar" : only match files whose name matches this regexp
	flagSearchPath = flag.String("f", "",
		"Search only in file names matching this regexp")
	flagInsens = flag.Bool("i", false, "Case insensitive search")

	// -key 1,2 -key 3 : one or more comma separated groups of keys
	flagKeys afind.FlagStringSlice
	flagMeta = make(afind.FlagSSMap)

	log = logging.MustGetLogger("afind")
)

func init() {
	flag.Var(&flagKeys, "key",
		"Search just this comma-separated list of repository keys")
	flag.Var(&flagMeta, "D",
		"A key=value pair to add to index or search request metadata")
}

// the union context for any single command execution
type ctx struct {
	repoKeys []string
	meta     map[string]string

	rpcClient *afind.RpcClient
}

func newContext() (*ctx, error) {
	if client, err := afind.NewRpcClient(*flagRpcAddress); err == nil {
		keys := make([]string, len(flagKeys))
		for i, k := range flagKeys {
			keys[i] = k
		}
		return &ctx{repoKeys: keys, rpcClient: client}, nil
	} else {
		return nil, err
	}
}

func werr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
}

func search(context *ctx, query string) error {
	request := afind.SearchRequest{
		Re:            query,
		PathRe:        *flagSearchPath,
		CaseSensitive: !*flagInsens,
		RepoKeys:      flagKeys,
		Meta:          flagMeta,
	}
	sr, err := context.rpcClient.Search(request)
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
	ir, err := context.rpcClient.Index(request)
	if err != nil {
		return err
	} else {
		fmt.Printf("indexed [%v] meta: %v in %v\n",
			ir.Repo.Key, ir.Repo.Meta, ir.Elapsed)
	}
	return nil
}

func printrepo(repo *afind.Repo) {
	fmt.Printf("Repo: %v\n", repo.Key)
	fmt.Printf("  root: %v\n", repo.Root)
	fmt.Printf("  file (directory) count: %d (%d)\n", repo.NumFiles, repo.NumDirs)
	fmt.Printf("  meta:\n")
	for mk, mv := range repo.Meta {
		fmt.Printf("    %s=%s\n", mk, mv)
	}
}

func repos(context *ctx, key string) error {
	var err error
	repos := make(map[string]*afind.Repo)
	repo, err := context.rpcClient.GetRepo(key)
	if repo == nil {
		// retrieve all repos, perhaps to filter
		rs, allerr := context.rpcClient.GetAllRepos()
		if allerr != nil {
			return allerr
		}
		for k, v := range rs {
			repos[k] = v
		}
	} else {

		repos[repo.Key] = repo
		fmt.Printf("%#v\n", repos)
	}

	for k, repo := range repos {
		if strings.HasPrefix(k, key) {
			printrepo(repo)
		}
	}

	return err
}

func doAfind() {
	command := strings.ToLower(flag.Arg(0))
	args := flag.Args()[1:]
	context, err := newContext()
	if err != nil {
		werr(err)
		return
	}
	switch command {
	case "index":
		// args, minimum 1
		// <repo key (prefix)> <repo root path> [subdir..]
		if len(args) < 2 {
			// usage
			return
		}
		key := args[0]
		root := args[1]
		subdirs := args[2:]
		err := index(context, key, root, subdirs)
		if err != nil {
			werr(err)
			return
		}
	case "search":
		if len(args) == 1 {
			search(context, args[0])
		} else {
			// invalid, print usage
		}
	case "repos":
		var key string
		if len(args) > 0 {
			key = args[0]
		}
		repos(context, key)
	default:
		// usage
	}
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
	common.LoggerStderr()

	if len(flag.Args()) < 1 {
		flag.Usage()
		return
	}
	doAfind()
}
