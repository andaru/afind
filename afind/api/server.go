package api

import (
	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/utils"
	"strings"
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

// NewServer creates a new base server from the components provided
func NewServer(rs afind.KeyValueStorer, ix afind.Indexer,
	sr afind.Searcher, c *afind.Config) *baseServer {
	return &baseServer{rs, ix, sr, *c}
}

// Utility functions used by servers

var (
	localHostnames = map[string]interface{}{
		"":          nil,
		"localhost": nil,
		// other hosts in 127/8 are not considered local,
		// allowing one host to test distributed requests
		"127.0.0.1": nil,
		"::1":       nil,
	}
)

// Does the host provided in the request match our local hostname?
func isLocal(config *afind.Config, h string) bool {
	if _, ok := localHostnames[h]; ok {
		return true
	} else if h == config.Host() {
		return true
	} else if strings.HasPrefix(config.Host(), h+".") {
		// allow the local domain to be stripped from repo
		// metadata and still match locally.
		return true
	}
	return false
}

func getAddress(meta afind.Meta, port string) string {
	if port == "" {
		port = ":" + utils.DefaultRpcPort
	} else if port[0] != ':' {
		port = ":" + port
	}
	return meta.Host() + port
}
