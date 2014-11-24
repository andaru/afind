package api

import (
	"github.com/andaru/afind/afind"
)

// baseServer is the base server implementation, carrying the Register*
// binding calls for the different interfaces making up the afind
// system that are bound to the servers
type baseServer struct {
	repos    afind.KeyValueStorer
	indexer  afind.Indexer
	searcher afind.Searcher
	config   afind.Config
}

// A virtual function for the HTTP/RPC servers to register their
// endpoints once the components are ready to go
func (s *baseServer) Register() {
}

// Setup everything in one go
func NewServer(rs afind.KeyValueStorer, ix afind.Indexer,
	sr afind.Searcher, c *afind.Config) *baseServer {
	return &baseServer{rs, ix, sr, *c}
}

func (s *baseServer) SetIndexer(ix afind.Indexer) {
	s.indexer = ix
}

func (s *baseServer) SetSearcher(sr afind.Searcher) {
	s.searcher = sr
}

func (s *baseServer) SetRepos(repos afind.KeyValueStorer) {
	s.repos = repos
}

func (s *baseServer) SetConfig(config afind.Config) {
	s.config = config
}

// Utility functions used by servers

var (
	localHostnames = map[string]interface{}{
		"":          nil,
		"localhost": nil,
		"127.0.0.1": nil,
		"::1":       nil,
	}
)

// Does the host provided in the request match our local hostname?
func isLocal(config *afind.Config, h string) bool {
	if _, ok := localHostnames[h]; ok {
		return true
	} else if config.Host() == h {
		return true
	}
	return false
}

func getAddress(meta afind.Meta, port string) string {
	if port == "" {
		port = ":30800"
	} else if port[0] != ':' {
		port = ":" + port
	}
	return meta.Host() + port
}
