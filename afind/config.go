package afind

import (
	"net"
	"os"
	"time"

	"github.com/andaru/afind/utils"
)

// Afind configuration file definition and handling

type Config struct {
	IndexInRepo bool   // the index file is placed in Repo's Root if true
	IndexRoot   string // path for index files if IndexInRepo is false
	HttpBind    string // HTTP bind address (e.g. ":80" or "0.0.0.0:80")
	HttpsBind   string // HTTPs bind address
	RpcBind     string // Gob RPC bind address
	NumShards   int    // number of index shards to create per Repo
	MaxSearchC  int    // Maximum search concurrency

	// Default index and search timeouts, in seconds
	// If not provided, the defaults below will be used, see
	// defaultTimeout* constants.
	TimeoutIndex  time.Duration
	TimeoutSearch time.Duration

	// Metadata to apply to every Repo created by an indexer with
	// this config. Manipulated by e.g., SetHost()
	RepoMeta map[string]string

	DbFile string // If non-empty, the JSON file containing the config backing store

	TlsCertfile string
	TlsKeyfile  string

	verbose bool
}

const (
	defaultTimeoutIndex  = 1800 * time.Second
	defaultTimeoutSearch = 30 * time.Second
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
