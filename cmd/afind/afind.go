package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/andaru/afind"
)

var (
	flagHttp = flag.Bool("http", false, "Run HTTP server")
	flagAddress = flag.String(
		"address", ":8080", "Local address:port for server")
	flagIndex      = flag.Bool("index", false, "Indexing mode")
	flagSearch     = flag.String("search", "", "Search regular expression")
	flagIndexFile  = flag.String("ixfile", "", "Index file name")
	flagKey        = flag.String("key", "", "Source shard key")
	flagSearchPath = flag.String("path", "", "Pathname regular expression")
)

func init() {
	flag.Parse()
}

func index() {
	exitOnBadArgs()

	absIndexFile, err := filepath.Abs(*flagIndexFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	src := afind.NewSourceWithPaths(
		*flagKey, absIndexFile, flag.Args())
	err = src.Index()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
	}
}

func exitOnBadArgs() {
	if *flagIndexFile == "" || *flagKey == "" {
		fmt.Fprintf(
			os.Stderr,
			"--ixfile and --key must be set\n")
		os.Exit(1)
	} else if *flagIndexFile == "" {
		fmt.Fprintf(
			os.Stderr,
			"--ixfile must be set\n")
		os.Exit(1)
	} else if *flagKey == "" {
		fmt.Fprintf(
			os.Stderr,
			"--key must be set\n")
		os.Exit(1)
	}
}

func search() {
	exitOnBadArgs()

	s := afind.NewSearcherFromIndex(*flagIndexFile)
	request := afind.NewSearchRequestWithPath(*flagSearch, *flagSearchPath)
	response, err := s.Search(request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
	} else {
		fmt.Println(len(response.M), "files match regexp", request.Re)
	}
}

func main() {
	if *flagHttp {
		server := afind.AfindServer()
		http.ListenAndServe(*flagAddress, server)
	} else {
		if *flagIndex {
			index()
		}
		if *flagSearch != "" {
			search()
		}
	}
}
