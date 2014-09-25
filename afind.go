package afind

import (
	"net"
	"net/http"
	"net/rpc"

	"github.com/gocraft/web"
	"github.com/golang/glog"
)

// This file composes the different system parts (defined as
// interfaces in data.go) into the overall Afind system.

type Backend interface {
	Indexer
	Searcher
}

type Service struct {
	repos    KeyValueStorer
	Indexer  indexer
	Searcher searcher
}

func newService(repos KeyValueStorer) *Service {
	return &Service{repos, indexer{repos}, searcher{repos}}
}

// The Afind system
type System struct {
	// Interfaces that comprise the system
	Indexer
	Searcher

	quit      chan error
	rpcServer *rpc.Server
	repos     KeyValueStorer
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
	var err error

	s.setupRpcServer()
	err = s.startRpcServer()
	if err != nil {
		glog.Error("Error starting RPC server:", err)
	}
	return err
}

func (s *System) startHttpServer() error {
	if config.BindFlag == "" {
		return nil
	}
	app := web.New(s)
	app.Get("/repos/:key", GetRepo)
	app.Post("/repos/:key", PostRepo)
	http.ListenAndServe(config.BindFlag, app)
	return nil
}

func (s *System) WaitForExit() error {
	return <-s.quit
}

func (s *System) setupRpcServer() {
	svc := newService(s.repos)
	rpcsvc := newRpcService(svc)
	s.rpcServer = rpc.NewServer()
	// Register a unified RPC interface serving all functions
	s.rpcServer.RegisterName("Afind", rpcsvc)
}

func (s *System) startRpcServer() error {
	if s.rpcServer == nil {
		panic("RPC server not initialized")
	}

	l, err := net.Listen("tcp", config.RpcBindFlag)
	if err == nil {
		glog.Info("Starting RPC server on", config.RpcBindFlag)
		go s.rpcServer.Accept(l)
	} else {
		s.rpcServer = nil
	}
	return err
}
