package afind

import (
	"os"
	"regexp"
	"time"
)

// Afind configuration file definition and handling

type Config struct {
	IndexInRepo bool   // the index file is placed in Repo's Root if true
	IndexRoot   string // path for index files if IndexInRepo
	HttpBind    string
	HttpsBind   string
	RpcBind     string
	NumShards   int // the number of Repo shards to build per index request

	timeoutIndex  int
	timeoutSearch int

	// default repository metadata
	// manipulated by e.g., DefaultHost()
	DefaultRepoMeta map[string]string

	noindexStr string // regular expression of files not to index
	noIndexRe  *regexp.Regexp

	DbFile string // If non-empty, the JSON file containing the config backing store
}

// The global configuration object
var (
	zzconfig *Config
)

const (
	defaultTimeoutIndex  = 1800 * time.Second
	defaultTimeoutSearch = 30 * time.Second
)

func (c *Config) TimeoutIndex() <-chan time.Time {
	if c.timeoutIndex == 0 {
		return time.After(defaultTimeoutIndex)
	}
	return time.After(time.Duration(c.timeoutIndex) * time.Second)
}

func (c *Config) TimeoutSearch() <-chan time.Time {
	if c.timeoutSearch == 0 {
		return time.After(defaultTimeoutSearch)
	}
	return time.After(time.Duration(c.timeoutSearch) * time.Second)
}

func (c *Config) DefaultPort() {
	if _, ok := c.DefaultRepoMeta["port.rpc"]; !ok {
		c.DefaultRepoMeta["port.rpc"] = c.RpcBind
	}
}

func (c *Config) DefaultHost() error {
	if _, ok := c.DefaultRepoMeta["host"]; ok {
		// 'host' already set, don't default
		return nil
	}
	hn, err := os.Hostname()
	if err == nil {
		c.DefaultRepoMeta["host"] = hn
	}
	return err
}

func (c *Config) SetNoIndex(reg string) error {
	re, err := regexp.Compile(c.noindexStr)
	if err == nil {
		c.noIndexRe = re
	}
	return err
}

func (c *Config) NoIndex() *regexp.Regexp {
	if c.noIndexRe == nil {
		_ = c.SetNoIndex(c.noindexStr)
	}
	return c.noIndexRe
}
