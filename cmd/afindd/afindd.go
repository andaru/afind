package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/afind/api"
	"github.com/andaru/afind/flags"
	"github.com/andaru/afind/utils"
)

const (
	periodTcpKeepAlive      = 3 * time.Minute
	defaultTimeoutIndex     = 1800.0
	defaultTimeoutSearch    = 30.0
	defaultTimeoutRepoStale = 12 * 3600.0
	defaultSearchParallel   = 40
	defaultSearchRepo       = 80
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
		IndexRoot:        *flagIndexRoot,
		IndexInRepo:      *flagIndexInRepo,
		HttpBind:         *flagHttpBind,
		HttpsBind:        *flagHttpsBind,
		RpcBind:          *flagRpcBind,
		NumShards:        *flagNumShards,
		RepoMeta:         afind.Meta(flagMeta),
		DbFile:           *flagDbFile,
		TimeoutIndex:     time.Duration(*flagTimeoutIndex * float64(time.Second)),
		TimeoutSearch:    time.Duration(*flagTimeoutSearch * float64(time.Second)),
		TimeoutRepoStale: time.Duration(*flagTimeoutRepoStale * float64(time.Second)),
		MaxSearchC:       *flagSearchPar,
		MaxSearchRepo:    *flagSearchRepo,
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
	flagVerbose      = flag.Bool("v", false, "Log verbosely")
	flagTimeoutIndex = flag.Float64("timeout_index", defaultTimeoutIndex,
		"The default indexing timeout, in seconds")
	flagTimeoutSearch = flag.Float64("timeout_search", defaultTimeoutSearch,
		"The default search timeout, in seconds")
	flagTimeoutRepoStale = flag.Float64("timeout_repo_stale", defaultTimeoutRepoStale,
		"How long to cache remote repo information")
	flagSearchPar = flag.Int("num_parallel", defaultSearchParallel,
		"Maximum concurrent searches operating at any one time")
	flagSearchRepo = flag.Int("num_repo", defaultSearchRepo,
		"Maximum number of repo to consult per query")
	flagMeta = make(flags.SSMap)

	log = utils.LoggerForModuleVerbose("afindd")
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

func crit(err error) {
	log.Critical("error: %v", err)
}

type system struct {
	repos    afind.KeyValueStorer
	indexer  afind.Indexer
	searcher afind.Searcher
	config   afind.Config

	quit chan struct{}
}

func newAfind() system {
	sys := system{config: getConfig()}
	if *flagDbFile != "" {
		log.Debug("writing repos in json to %v", *flagDbFile)
		sys.repos = afind.NewJsonBackedDb(*flagDbFile)
	} else {
		log.Debug("no backing store; repos stored in process memory only")
		sys.repos = afind.NewDb()
	}
	sys.indexer = afind.NewIndexer(&sys.config, sys.repos)
	sys.searcher = afind.NewSearcher(&sys.config, sys.repos)
	return sys
}

func main() {
	log.Info("afindd daemon starting")
	flag.Parse()
	af := newAfind()

	cfg := &af.config
	server := api.NewServer(af.repos, af.indexer, af.searcher, cfg)

	// setup quit signal channel (aka handler)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	if cfg.RpcBind != "" {
		log.Info("rpc server start [%v]", cfg.RpcBind)
		if l, err := cfg.ListenerRpc(); err == nil {
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
		s := api.NewWebServer(server)
		s.Register()
		go func() {
			err := s.HttpServer(cfg.HttpBind).ListenAndServe()
			if err != nil {
				crit(err)
			}
		}()
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

// A modified TCP listener that sets the TCP keep-alive to eventually
// timeout abandoned client connections. Modified from the version in
// golang's standard library (net/http/server.go).
type tcpKeepAliveListener struct {
	*net.TCPListener
	timeout time.Duration
}

func newTcpKeepAliveListener(l net.Listener, t time.Duration) tcpKeepAliveListener {
	return tcpKeepAliveListener{l.(*net.TCPListener), t}
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	_ = tc.SetKeepAlive(true)
	_ = tc.SetKeepAlivePeriod(ln.timeout)
	return tc, nil
}
