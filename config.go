package afind

import (
	"os"
	"regexp"
)

// Afind configuration file definition and handling

type Config struct {
	IndexInRepo bool   // the index file is placed in Repo's Root if true
	IndexRoot   string // path for index files if IndexInRepo
	HttpBind    string
	HttpsBind   string
	RpcBind     string
	NumShards   int // the number of Repo shards to build per index request

	// default repository metadata
	// manipulated by e.g., DefaultHostname()
	DefaultRepoMeta map[string]string

	noindexStr string // regular expression of files not to index
	noIndexRe  *regexp.Regexp
}

// The global configuration object
var (
	config *Config
)

func init() {
	config := &Config{DefaultRepoMeta: make(map[string]string)}
}

func (c *Config) DefaultHostname() error {
	if _, ok := c.DefaultRepoMeta["hostname"]; ok {
		// 'hostname' already set, don't default
		return nil
	}
	hn, err := os.Hostname()
	if err == nil {
		c.DefaultRepoMeta["hostname"] = hn
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

func SetConfig(c Config) {
	config = &c
}

func GetConfig() *Config {
	return config
}
