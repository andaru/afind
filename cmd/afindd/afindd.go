package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/andaru/afind"
	"github.com/op/go-logging"
)

var (
	log = logging.MustGetLogger("afindd")
)

// see flags.go for flag definitions

func setupConfig() *afind.Config {
	c := &afind.Config{
		IndexRoot:       *flagIndexRoot,
		IndexInRepo:     *flagIndexInRepo,
		HttpBind:        *flagHttpBind,
		HttpsBind:       *flagHttpsBind,
		RpcBind:         *flagRpcBind,
		NumShards:       *flagNumShards,
		DefaultRepoMeta: make(map[string]string),
		DbFile:          *flagDbFile,
		LogVerbose:      *flagVerbose,
	}
	// Update any default metadata provided at the commandline
	for k, v := range flagMeta {
		c.DefaultRepoMeta[k] = v
	}
	// Provide a default hostname from the OS, else "localhost"
	_ = c.DefaultHost()
	// Provide the RPC port as port.rpc in the metadata
	c.DefaultPort()
	// Setup and cache the "no indexing" path regular expression
	_ = c.SetNoIndex(*flagNoIndex)
	// Apply the configuration to the process
	log.Debug("afind configuration %+v", c)

	// Create index path
	if !c.IndexInRepo {
		if err := os.MkdirAll(c.IndexRoot, 0750); err != nil {
			log.Fatal(err.Error())
		}
	}

	return c
}

func setupLogging() {
	var level logging.Level
	if *flagVerbose {
		level = logging.DEBUG
	} else {
		level = logging.INFO
	}

	leveled := logging.AddModuleLevel(logging.NewLogBackend(os.Stderr, "", 0))
	leveled.SetLevel(level, "afind")
	leveled.SetLevel(level, "afindd")
	logging.SetBackend(leveled)
	format := logging.MustStringFormatter(
		"%{color:bold}%{level:.1s}%{time:0102 15:04:05.999999} " +
			"%{pid} %{shortfunc} %{shortfile}]%{color:reset} %{message}")
	logging.SetFormatter(format)
}

var af *afind.System

func main() {
	var err error

	flag.Parse()

	setupLogging()
	config := setupConfig()
	af := afind.New(*config)
	af.Start()

	err = af.WaitForExit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(3)
	}
}
