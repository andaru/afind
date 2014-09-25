package main

import (
	"flag"
	"fmt"

	"github.com/andaru/afind"
	"strings"
)

var (
	flagHttp    = flag.Bool("http", false, "Run HTTP server")
	flagAddress = flag.String(
		"address", ":8080", "Local address:port for server")
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

func search(query string) {
	fmt.Printf("Searching for '%s'", query)
	key := *flagKey
	if key == "" {
		fmt.Println()
	} else {
		fmt.Println(" in repo", key)
	}
}

func main() {
	if len(flag.Args()) < 1 {
		flag.Usage()
		return
	}

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
		fmt.Println("foo:", args)
		if len(args) == 1 {
			// valid
			request := afind.SearchRequest{
				Re:     args[0],
				PathRe: *flagSearchPath,
			}
			fmt.Printf("searching %#v\n", request)
			fmt.Println(context.Search(request))
		} else {
			// invalid
		}
	default:
		fmt.Errorf("Unknown command '%s'\n", command)
	}
}

func newContext() *ctx {
	client, err := afind.NewRpcClient(*flagAddress)
	fmt.Println(client)
	if err != nil {
		fmt.Printf("Failed to connect to server '%s'\n", *flagAddress)
		client = nil
	}
	return &ctx{
		rpcSvrAddress: *flagAddress,
		repoKey:       *flagKey,
		rpcClient:     client,
	}
}

type ctx struct {
	rpcSvrAddress string
	repoKey       string
	rpcClient     *afind.RpcClient
}
