package main

import (
	"flag"
	"os"

	"fmt"
	"github.com/andaru/afind"
	"github.com/op/go-logging"
)

var (
	flagIndexRoot = flag.String(
		"index_root", "/tmp/afind", "Index file path")
	flagIndexInRepo = flag.Bool("index_in_repo", true,
		"Write indices to -index_root if false, else in repository root path")
	flagNoIndex = flag.String("noindex", "",
		"A regexp matching file names to not index")
	flagRpcBind   = flag.String("rpc", ":30800", "RPC server bind addr")
	flagHttpBind  = flag.String("http", ":30880", "HTTP server bind addr")
	flagHttpsBind = flag.String("https", ":30880", "HTTP server bind addr")
	flagNumShards = flag.Int("nshards", 4,
		"Maximum number of Repo shards created per indexing request")
	flagMeta = make(afind.FlagSSMap)

	log = logging.MustGetLogger("afindd")
)

func init() {
	// setup the -D default metadata flag.
	// This is used by the client to set fields such as the hostname,
	// which will default to the hostname reported by the kernel.
	flag.Var(&flagMeta, "D",
		"A key=value default repository metadata field (may be repeated)")
}

func setupConfig() {
	c := afind.Config{
		IndexRoot:       *flagIndexRoot,
		IndexInRepo:     *flagIndexInRepo,
		HttpBind:        *flagHttpBind,
		HttpsBind:       *flagHttpsBind,
		RpcBind:         *flagRpcBind,
		NumShards:       *flagNumShards,
		DefaultRepoMeta: make(map[string]string),
	}
	// Update any default metadata provided at the commandline
	for k, v := range flagMeta {
		c.DefaultRepoMeta[k] = v
	}
	// Provide a default hostname from the OS, else "localhost"
	_ = c.DefaultHost()
	// Setup and cache the "no indexing" path regular expression
	_ = c.SetNoIndex(*flagNoIndex)
	// Apply the configuration to the process
	log.Debug("configuration %#v", c)
	afind.SetConfig(c)
}

func setupLogging() {
	logger := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(logger)
	logging.SetFormatter(logging.GlogFormatter)
}

var af *afind.System

func main() {
	var err error
	flag.Parse()
	setupLogging()
	setupConfig()

	af := afind.New()
	af.Start()
	err = af.WaitForExit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(3)
	}
}
