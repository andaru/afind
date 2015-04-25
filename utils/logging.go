package utils

import (
	"os"

	"github.com/op/go-logging"
)

var (
	leveled logging.LeveledBackend
)

func LogToFile(module, path string, verbose bool) *logging.Logger {
	var f *os.File
	var err error

	if path == "-" {
		f = os.Stdout
	} else {
		if path == "" {
			path = os.DevNull
		}
		f, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			panic("cannot write log file: " + err.Error())
		}
	}

	be := logging.NewLogBackend(f, "", 0)
	leveled = logging.AddModuleLevel(be)
	logging.SetBackend(leveled)
	result := logging.MustGetLogger(module)
	if verbose {
		leveled.SetLevel(logging.DEBUG, module)
	} else {
		leveled.SetLevel(logging.INFO, module)
	}
	return result
}

func Logger(module string) *logging.Logger {
	return logging.MustGetLogger(module)
}

func init() {
	setupLogging()
}

func setupLogging() {
	format := logging.MustStringFormatter(
		"%{level:.1s}%{time:0102 15:04:05.999999} " +
			"%{pid} %{shortfunc} %{shortfile}] %{message}")
	logging.SetFormatter(format)
}
