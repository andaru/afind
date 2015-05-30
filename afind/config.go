package afind

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/andaru/afind/utils"
)

// Config holds the afindd instance live configuration
type Config struct {
	IndexInRepo       bool   // the index file is placed in Repo's Root if true
	IndexRoot         string // path for index files if IndexInRepo is false
	HTTPBind          string // HTTP bind address (e.g. ":80" or "0.0.0.0:80")
	HTTPSBind         string // HTTPs bind address
	RPCBind           string // Gob RPC bind address
	NumShards         int    // number of index shards to create per Repo
	MaxSearchC        int    // Maximum search concurrency
	MaxSearchRepo     int    // Maximum number of Repo to consider per search
	MaxSearchReqBe    int    // Maximum number of backend requests per query
	DeleteRepoOnError bool   // If True, delete Repo from afindd on ERROR

	// Default index, search and find timeouts, in seconds
	// If not provided, the defaults below will be used, see
	// defaultTimeout* constants.
	TimeoutIndex  time.Duration
	TimeoutSearch time.Duration
	TimeoutFind   time.Duration

	// TCP keepalive timeout for all server sockets
	TimeoutTcpKeepAlive time.Duration

	// Metadata to apply to every Repo created by an indexer with
	// this config. Manipulated by e.g., SetHost()
	RepoMeta Meta

	DbFile string // If non-empty, the JSON file containing the config backing store

	TLSCertfile string
	TLSKeyfile  string

	verbose bool
}

const (
	defaultTimeoutIndex        = 1800 * time.Second
	defaultTimeoutSearch       = 30 * time.Second
	defaultTimeoutFind         = 500 * time.Millisecond
	defaultTimeoutTcpKeepAlive = 3 * time.Minute
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

func (c *Config) GetTimeoutFind() time.Duration {
	if c.TimeoutFind == 0 {
		c.TimeoutFind = defaultTimeoutFind
	}
	return c.TimeoutFind
}

func (c *Config) GetTimeoutTcpKeepAlive() time.Duration {
	if c.TimeoutTcpKeepAlive == 0 {
		c.TimeoutTcpKeepAlive = defaultTimeoutTcpKeepAlive
	}
	return c.TimeoutTcpKeepAlive
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
	return net.Listen("tcp", c.RPCBind)
}

func (c *Config) ListenerTcpWithTimeout(
	addr string,
	timeout time.Duration) (l net.Listener, err error) {

	if l, err = net.Listen("tcp", addr); err == nil {
		return newTcpKeepAliveListener(l, timeout), err
	}
	return
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
