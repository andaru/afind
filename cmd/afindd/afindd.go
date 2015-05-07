package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/afind/api"
	"github.com/andaru/afind/flags"
	"github.com/andaru/afind/utils"
	"github.com/op/go-logging"
)

const (
	defaultTcpKeepAlive   = 3 * time.Minute
	defaultTimeoutIndex   = 30 * time.Minute
	defaultTimeoutSearch  = 30 * time.Second
	defaultTimeoutFind    = 5000 * time.Millisecond
	defaultSearchParallel = 200
	defaultSearchRepo     = 0
	defaultSearchReqBe    = 300
)

func init() {
	// setup the -D default metadata flag.
	// This is used by the client to set fields such as the hostname,
	// which will default to the hostname reported by the kernel.
	flag.Var(&flagMeta, "D",
		"A key=value metadata attribute to write on all indexed repos")
	flag.Usage = usage
}

func getConfig() afind.Config {
	c := afind.Config{
		IndexRoot:           *flagIndexRoot,
		IndexInRepo:         *flagIndexInRepo,
		HttpBind:            *flagHttpBind,
		HttpsBind:           *flagHttpsBind,
		RpcBind:             *flagRpcBind,
		NumShards:           *flagNumShards,
		RepoMeta:            afind.Meta(flagMeta),
		DbFile:              *flagDbFile,
		TimeoutIndex:        *flagTimeoutIndex,
		TimeoutSearch:       *flagTimeoutSearch,
		TimeoutTcpKeepAlive: *flagTimeoutTcpKeepAlive,
		TimeoutFind:         *flagTimeoutFind,
		MaxSearchC:          *flagSearchPar,
		MaxSearchRepo:       *flagSearchRepo,
		MaxSearchReqBe:      *flagSearchReqBe,
	}
	c.SetVerbose(*flagVerbose)
	c.Host()
	return c
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
	flagHttpBind = flag.String("http", "",
		"Run HTTP server on this address:port")
	flagHttpsBind = flag.String("https", "",
		"Run HTTPS server on this address:port")
	flagNumShards = flag.Int("nshards", 4,
		"Number of file shards created per Repo indexing request")
	flagDbFile = flag.String("dbfile", "",
		"The Repo persistent storage backing (JSON)")
	flagVerbose = flag.Bool("v", false,
		"Log verbosely")
	flagTimeoutIndex = flag.Duration("timeout_index", defaultTimeoutIndex,
		"The default indexing timeout, a duration")
	flagTimeoutSearch = flag.Duration("timeout_search", defaultTimeoutSearch,
		"The default search timeout, a duration")
	flagTimeoutFind = flag.Duration("timeout_find", defaultTimeoutFind,
		"The default find timeout, a duration")
	flagTimeoutTcpKeepAlive = flag.Duration("timeout_tcp_keepalive", defaultTcpKeepAlive,
		"The default TCP keepalive timeout for server sockets, a duration")
	flagSearchPar = flag.Int("num_parallel", defaultSearchParallel,
		"Maximum concurrent searches operating at any one time")
	flagSearchRepo = flag.Int("num_repo", defaultSearchRepo,
		"Maximum number of repo to consult per query")
	flagSearchReqBe = flag.Int("num_request_be", defaultSearchReqBe,
		"Maximum number of backend requests per query")
	flagLogPath = flag.String("log", os.DevNull,
		"Log to this path (use - for stdout)")
	flagMeta = make(flags.SSMap)

	log *logging.Logger
)

func setupLogging() {
	utils.SetLevel("INFO")
	if *flagVerbose {
		utils.SetLevel("DEBUG")
	}
	log = utils.LogToFile("afind", *flagLogPath)
}

func usage() {
	fmt.Fprintln(os.Stderr,
		`afindd : distributed text search server daemon

Usage:
  afindd [options]

afindd does not fork and does not, by default write logs. Use the
-log and -v flags to control logging destination and verbosity.

Options:`)
	flag.PrintDefaults()
}

func crit(err error) {
	log.Critical("error: %v", err)
}

type system struct {
	repos    afind.KeyValueStorer
	indexer  afind.Indexer
	searcher afind.Searcher
	finder   afind.Finder
	config   afind.Config

	quit chan struct{}
}

func newAfind(cfg afind.Config) system {
	sys := system{config: cfg}
	if *flagDbFile != "" {
		sys.repos = afind.NewJsonBackedDb(*flagDbFile)
	} else {
		log.Warning("no repo backing store - repos will be lost at process exit")
		sys.repos = afind.NewDb()
	}
	sys.indexer = afind.NewIndexer(&sys.config, sys.repos)
	sys.searcher = afind.NewSearcher(&sys.config, sys.repos)
	sys.finder = afind.NewFinder(&sys.config, sys.repos)
	return sys
}

func main() {
	flag.Parse()
	cfg := getConfig()
	setupLogging()
	log.Info("afindd daemon starting")
	af := newAfind(cfg)
	server := api.NewServer(af.repos, af.indexer, af.searcher, af.finder, &cfg)

	// setup quit signal channel (aka handler)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	if cfg.RpcBind != "" {
		log.Info("rpc server start [%v]", cfg.RpcBind)
		if l, err := cfg.ListenerTcpWithTimeout(
			cfg.RpcBind, cfg.GetTimeoutTcpKeepAlive()); err == nil {

			s := api.NewRpcServer(l, server)
			s.Register()

			go func() {
				defer s.CloseNoErr()
				err = s.Serve()
				if err != nil {
					crit(err)
				}
			}()
		} else {
			crit(err)
		}
	}

	if cfg.HttpBind != "" {
		log.Info("http server start [%v]", cfg.HttpBind)
		if l, err := cfg.ListenerTcpWithTimeout(
			cfg.HttpBind, cfg.GetTimeoutTcpKeepAlive()); err == nil {

			s := api.NewWebServer(server)
			s.Register()
			go func() {
				httpd := s.HttpServer(cfg.HttpBind)
				err := httpd.Serve(l)
				if err != nil {
					crit(err)
				}
			}()
		} else {
			crit(err)
		}
	}

	if cfg.HttpsBind != "" {
		log.Info("https server start [%v]", cfg.HttpsBind)
		s := api.NewWebServer(server)
		s.Register()
		go func() {
			err := s.HttpServer(cfg.HttpsBind).ListenAndServeTLS(
				cfg.TlsCertfile, cfg.TlsKeyfile)
			if err != nil {
				crit(err)
			}
		}()
	}

	// remain running awaiting a signal
	sig := <-quit
	if sig != nil {
		log.Info("exiting due to %s", sig)
	}
}
