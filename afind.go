package afind

import (
	"os"
)

// This file composes the different system parts (defined as
// interfaces in data.go) into the overall Afind system.

// The Service components in the Afind system, used
// by server interfaces such as the HTTP(S) WebService
// or the Gob RPC over TCP RpcService
type Service struct {
	config   Config
	repos    KeyValueStorer
	remotes  Remotes
	Indexer  Indexer
	Searcher Searcher
}

func NewService(repos KeyValueStorer, c Config) *Service {
	svc := Service{
		config:  c,
		repos:   repos,
		remotes: NewRemotes(),
	}
	svc.Indexer = newIndexer(svc)
	svc.Searcher = newSearcher(svc)
	return &svc
}

// The Afind system
type System struct {
	service    Service
	quit       chan error
	rpcService *rpcService
	webService *webService
}

// Composes and returns a new Afind system.
// Should only be called once per program execution.
//
// Test code can call this function with test implementations of the
// interfaces used in the system struct.
func composeSystem(service Service) *System {
	return &System{
		service:    service,
		quit:       make(chan error),
		rpcService: newRpcService(&service),
		webService: newWebService(&service),
	}
}

// The system creation entry point for the afind system.
func New(c Config) *System {
	var repos KeyValueStorer

	if c.DbFile == "" {
		repos = newDb()
	} else {
		repos = newDbWithJsonBacking(c.DbFile)
	}

	service := NewService(repos, c)
	if err := makeIndexRoot(c); err != nil {
		log.Fatalf("Could not make IndexRoot: %v", err)
	}

	return composeSystem(*service)
}

func (s *System) Start() {
	var err error

	err = s.rpcService.start()
	if err != nil {
		log.Fatalf("RPC server error: %v", err)
	}

	err = s.webService.start()
	if err != nil {
		log.Fatalf("Web server error: %v", err)
	}
	log.Info("Afind system started")
}

func (s *System) ExitWithError(err error) {
	go func() {
		s.quit <- err
	}()
}

func (s *System) WaitForExit() error {
	return <-s.quit
}

func makeIndexRoot(config Config) error {
	return os.MkdirAll(config.IndexRoot, 0750)
}
