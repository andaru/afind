package main

import (
	"flag"

	"github.com/andaru/afind"
)

var (
	indexRoot   = flag.String("index_root", "/tmp", "Index path")
	indexInRepo = flag.Bool("index_in_repo", true,
		"Whether to prefix --index_root with the repo root directory")
	noIndex     = flag.String("noindex", "", "Filename regexp to not index")
	rpcBindFlag = flag.String("rpc_bind", ":30800", "RPC server bind addr")
	bindFlag    = flag.String("bind", ":8088", "HTTP server bind addr")
)

func init() {
	flag.Parse()
	c := afind.Config{
		Noindex:     *noIndex,
		IndexRoot:   *indexRoot,
		IndexInRepo: *indexInRepo,
		RpcBindFlag: *rpcBindFlag,
		BindFlag:    *bindFlag,
	}
	// Update the system configuration
	afind.SetConfig(c)
}

func main() {
	a := afind.New()
	a.Start()
	a.WaitForExit()
}
