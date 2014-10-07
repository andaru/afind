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

	log = logging.MustGetLogger("afind")
)

func init() {
	flag.Var(&flagKeys, "key",
		"Search just this comma-separated list of repository keys")
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

func search(context *ctx, query string) {
	request := afind.SearchRequest{
		Re:            query,
		PathRe:        *flagSearchPath,
		CaseSensitive: !*flagInsens,
	}
	sr, err := context.rpcClient.Search(request)
	if err != nil {
		fmt.Errorf("error: %v\n", err)
	}
	printMatches(sr)
}

func werr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
}

func repos(context *ctx, key string) {
	if key == "" {
		// show list of repos
		repos, err := context.rpcClient.GetAllRepos()
		if err != nil {
			werr(err)
			return
		}
		for k, v := range repos.Repos {
			fmt.Printf("Repo: %v\n", k)
			fmt.Printf("  root: %v\n", v.Root)
			fmt.Printf("  file (directory) count: %d (%d)\n", v.NumFiles, v.NumDirs)
			fmt.Printf("  meta:\n")
			for mk, mv := range v.Meta {
				fmt.Printf("    %s=%s\n", mk, mv)
			}
			fmt.Println()
		}
	}
}

func index(context *ctx, key, root string, subdirs []string) error {
	request := afind.IndexRequest{Key: key, Root: root, Dirs: subdirs}
	ir, err := context.rpcClient.Index(request)
	if err != nil {
		werr(err)
		return err
	} else {
		r, ok := ir.Repos[key]
		if ok {
			fmt.Printf("indexed [%v] meta: %v in %v\n",
				key, r.Meta, ir.Elapsed)
		}

	}
	return nil
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
