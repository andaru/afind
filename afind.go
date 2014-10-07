package afind

import (
	"net"
	"net/http"
	"net/rpc"

	"github.com/gocraft/web"
)

// This file composes the different system parts (defined as
// interfaces in data.go) into the overall Afind system.

type Service struct {
	repos    KeyValueStorer
	Indexer  indexer
	Searcher searcher
}

func newService(repos KeyValueStorer) *Service {
	return &Service{repos, *newIndexer(repos), *newSearcher(repos)}
}

// The Afind system
type System struct {
	// Interfaces that comprise the system
	Indexer
	Searcher

	quit       chan error
	rpcServer  *rpc.Server
	httpServer *http.Server
	repos      KeyValueStorer
}

// Composes and returns a new Afind system.
// Should only be called once per program execution.
//
// Test code can call this function with test implementations of the
// interfaces used in the system struct.
func composeSystem(repos KeyValueStorer, i Indexer, s Searcher) *System {
	return &System{
		Indexer:  i,
		Searcher: s,
		quit:     make(chan error),
		repos:    repos,
	}
}

// The system creation entry point for the afind system.
func New() *System {
	repos := newDb()
	indexer := newIndexer(repos)
	searcher := newSearcher(repos)
	return composeSystem(repos, indexer, searcher)
}

func (s *System) Start() error {
	// var err error
	// err = s.startHttpServer()
	// if err != nil {
	// 	glog.Error("Error starting HTTP server:", err)
	// 	return err
	// }
	s.setupRpcServer()
	return s.startRpcServer()
}

func (s *System) startHttpServer() error {
	if config.HttpBind == "" {
		return nil
	}
	app := web.New(s)
	app.Get("/repos/:key", GetRepo)
	app.Post("/repos/:key", PostRepo)
	go http.ListenAndServe(config.HttpBind, app)
	return nil
}

func (s *System) WaitForExit() error {
	return <-s.quit
}

func (s *System) setupRpcServer() {
	svc := newService(s.repos)
	rpcsvc := newRpcService(svc)
	s.rpcServer = rpc.NewServer()
	if err := s.rpcServer.RegisterName("Afind", rpcsvc); err != nil {
		log.Fatal(err)
	}

}

func (s *System) startRpcServer() error {
	if s.rpcServer == nil {
		s.setupRpcServer()
	}

	l, err := net.Listen("tcp", config.RpcBind)
	if err == nil {
		log.Info("rpc server started, bound to '%s'", config.RpcBind)
		go s.rpcServer.Accept(l)
	} else {
		s.rpcServer = nil
	}
	return err
}
