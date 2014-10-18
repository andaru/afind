package afind

import (
	"os"
	"regexp"
	"strings"
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

	// Default index and search timeouts, in seconds
	TimeoutIndex  float64
	TimeoutSearch float64

	// default repository metadata
	// manipulated by e.g., DefaultHost()
	DefaultRepoMeta map[string]string

	noindexStr string // regular expression of files not to index
	noIndexRe  *regexp.Regexp

	DbFile     string // If non-empty, the JSON file containing the config backing store
	LogVerbose bool   // Whether to log verbosely
}

const (
	defaultTimeoutIndex  = 1800 * time.Second
	defaultTimeoutSearch = 30 * time.Second
)

func (c *Config) GetTimeoutIndex() <-chan time.Time {
	if c.TimeoutIndex == 0 {
		c.TimeoutIndex = float64(defaultTimeoutIndex) / float64(time.Second)
	}
	return time.After(time.Duration(c.TimeoutIndex) * time.Second)
}

func (c *Config) GetTimeoutSearch() <-chan time.Time {
	if c.TimeoutSearch == 0 {
		c.TimeoutSearch = float64(defaultTimeoutSearch) / float64(time.Second)
	}
	return time.After(time.Duration(c.TimeoutSearch) * time.Second)
}

func (c *Config) DefaultPort() {
	if _, ok := c.DefaultRepoMeta["port.rpc"]; !ok {
		port := c.RpcBind[strings.Index(c.RpcBind, ":")+1:]
		c.DefaultRepoMeta["port.rpc"] = port
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
