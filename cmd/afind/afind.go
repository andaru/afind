package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/andaru/afind"
)

var (
	flagHttp       = flag.Bool("http", false, "Run HTTP server")
	flagInsens     = flag.Bool("i", false, "Case insensitive search")
	flagAddress    = flag.String("address", ":30800", "Connect to")
	flagIndex      = flag.Bool("index", false, "Indexing mode")
	flagSearch     = flag.String("search", "", "Search regular expression")
	flagIndexFile  = flag.String("ixfile", "", "Index file name")
	flagKey        = flag.String("key", "", "Source shard key")
	flagSearchPath = flag.String("f", "", "Pathname regular expression")
	flagMaster     = flag.Bool("master", false, "Master mode (default: slave)")
)

func init() {
	flag.Parse()
}

func search(context *ctx, query string) {
	request := afind.SearchRequest{
		Re:     query,
		PathRe: *flagSearchPath,
	}
	sr, err := context.rpcClient.Search(request)
	if err != nil {
		fmt.Errorf("Error: %v\n", err)
	}
	printMatches(sr)
}

func doAfind() {
	command := strings.ToLower(flag.Arg(0))
	args := flag.Args()[1:]
	context := newContext()
	if context.rpcClient == nil {
		return
	}

	switch command {
	case "index":
		fmt.Println("index")
	case "search":
		if len(args) == 1 {
			search(context, args[0])
		} else {
			// invalid, print usage
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
	if len(flag.Args()) < 1 {
		flag.Usage()
		return
	}
	doAfind()
}

func newContext() *ctx {
	client, err := afind.NewRpcClient(*flagAddress)
	if err != nil {
		fmt.Printf("Failed to connect to server '%s'\n", *flagAddress)
		client = nil
	}
	return &ctx{
		repoKey:   *flagKey,
		rpcClient: client,
	}
}

type ctx struct {
	repoKey   string
	rpcClient *afind.RpcClient
}
