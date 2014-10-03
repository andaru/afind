package main

import (
	"os"
	"flag"
	"fmt"
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
		Re:     query,
		PathRe: *flagSearchPath,
	}
	sr, err := context.rpcClient.Search(request)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	printMatches(sr)
}

func repos(context *ctx) {
}

func index(context *ctx, key, root string, subdirs []string) error {
	return nil
}

func doAfind() {
	command := strings.ToLower(flag.Arg(0))
	args := flag.Args()[1:]
	context, err := newContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, 
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

		}

	case "search":
		if len(args) == 1 {
			search(context, args[0])
		} else {
			// invalid, print usage
		}
	case "repos":
		if len(args) > 0 {
		}
	default:
		// usage
	}
}

func printMatches(sr *afind.SearchResponse) {
	for name, repos := range sr.Files {
		for repo, matches := range repos {
			for l, text := range matches {
				fmt.Printf("%s:%s:%s:%s\n",
					name, repo, l, text)
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
