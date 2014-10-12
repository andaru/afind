package main

import (
	"flag"
	"github.com/andaru/afind"
)

func init() {
	// setup the -D default metadata flag.
	// This is used by the client to set fields such as the hostname,
	// which will default to the hostname reported by the kernel.
	flag.Var(&flagMeta, "D",
		"A key=value default repository metadata field (may be repeated)")
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
	flagNanomsgBind = flag.String("nanomsg", ":30801",
		"Run nanomsg (mangos) pubsub server on this address:port")
	flagNumShards = flag.Int("nshards", 2,
		"Number of Repo shards created per indexing request")
	flagDbFile = flag.String("dbfile", "",
		"The database (JSON) backing store")
	flagMeta = make(afind.FlagSSMap)
)
