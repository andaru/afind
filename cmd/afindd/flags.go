package main

import (
	"flag"
	"fmt"
	"github.com/andaru/afind"
	"os"
)

func init() {
	// setup the -D default metadata flag.
	// This is used by the client to set fields such as the hostname,
	// which will default to the hostname reported by the kernel.
	flag.Var(&flagMeta, "D",
		"A key=value default repository metadata field (may be repeated)")
	flag.Usage = usage
}

var (
	flagIndexRoot = flag.String(
		"index_root", "/tmp/afind", "Index file path")
	flagIndexInRepo = flag.Bool("index_in_repo", true,
		"Write indices to -index_root if false, else in repository root path")
	flagNoIndex = flag.String("noindex", "",
		"A regexp matching file names to skip for indexing")
	flagRpcBind = flag.String("rpc", ":30800",
		"Run RPC server on this address:port")
	flagHttpBind = flag.String("http", ":30880",
		"Run HTTP server on this address:port")
	flagHttpsBind = flag.String("https", ":30880",
		"Run HTTPS server on this address:port")
	flagNumShards = flag.Int("nshards", 4,
		"Number of file shards created per Repo indexing request")
	flagDbFile = flag.String("dbfile", "",
		"The Repo persistent storage backing (JSON)")
	flagVerbose      = flag.Bool("v", false, "Log verbosely")
	flagTimeoutIndex = flag.Float64("timeout", 600.0,
		"Set the default indexing timeout in seconds")
	flagMeta = make(afind.FlagSSMap)
)

func usage() {
	fmt.Fprintln(os.Stderr,
		`afindd : distributed text search server daemon

Usage:
  afindd [options]

afindd does not fork, and writes logs to stderr.

Options:`)
	flag.PrintDefaults()
}
