package afind

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/andaru/afind/utils"
)

// Afind configuration file definition and handling

type Config struct {
	IndexInRepo   bool   // the index file is placed in Repo's Root if true
	IndexRoot     string // path for index files if IndexInRepo is false
	HttpBind      string // HTTP bind address (e.g. ":80" or "0.0.0.0:80")
	HttpsBind     string // HTTPs bind address
	RpcBind       string // Gob RPC bind address
	NumShards     int    // number of index shards to create per Repo
	MaxSearchC    int    // Maximum search concurrency
	MaxSearchRepo int    // Maximum number of Repo to consider per search

	// Default index and search timeouts, in seconds
	// If not provided, the defaults below will be used, see
	// defaultTimeout* constants.
	TimeoutIndex  time.Duration
	TimeoutSearch time.Duration

	TimeoutRepoStale time.Duration

	// Metadata to apply to every Repo created by an indexer with
	// this config. Manipulated by e.g., SetHost()
	RepoMeta Meta

	DbFile string // If non-empty, the JSON file containing the config backing store

	TlsCertfile string
	TlsKeyfile  string

	verbose bool
}

const (
	defaultTimeoutIndex  = 1800 * time.Second
	defaultTimeoutSearch = 30 * time.Second
)

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

func (c *Config) SetVerbose(verbose bool) {
	c.verbose = verbose
}

func (c *Config) Verbose() bool {
	return c.verbose
}

func (c *Config) GetTimeoutIndex() time.Duration {
	if c.TimeoutIndex == 0 {
		c.TimeoutIndex = defaultTimeoutIndex
	}
	return c.TimeoutIndex
}

func (c *Config) GetTimeoutSearch() time.Duration {
	if c.TimeoutSearch == 0 {
		c.TimeoutSearch = defaultTimeoutSearch
	}
	return c.TimeoutSearch
}

func (c *Config) PortRpc() (port string) {
	port = c.RepoMeta["port.rpc"]
	if port == "" {
		port = utils.DefaultRpcPort
	}
	return
}

func (c *Config) Host() string {
	if host := c.RepoMeta["host"]; host != "" {
		return host
	}

	hn, err := os.Hostname()
	if err != nil {
		hn = "localhost"
	}
	c.RepoMeta["host"] = hn
	return hn
}

func (c *Config) ListenerRpc() (l net.Listener, err error) {
	return net.Listen("tcp", c.RpcBind)
}

// IsHostLocal returns whether the passed in hostname is
// considered to be local to this machine
func (c *Config) IsHostLocal(host string) bool {
	if _, ok := localHostnames[host]; ok {
		return true
	} else if host == c.Host() {
		return true
	} else if strings.HasPrefix(c.Host(), host+".") {
		// allow the local domain to be stripped from repo
		// metadata and still match locally.
		return true
	}
	return false
}
