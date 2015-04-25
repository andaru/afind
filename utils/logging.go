package utils

import (
	"os"

	"github.com/op/go-logging"
)

var (
	leveled logging.LeveledBackend
	level   logging.Level
)

func init() {
	// set DEBUG level for testing
	level = logging.DEBUG
}

func LogToFile(module, path string) *logging.Logger {
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
	leveled.SetLevel(level, module)
	logging.SetBackend(leveled)
	result := logging.MustGetLogger(module)
	return result
}

func SetLevel(newlevel string) {
	l, err := logging.LogLevel(newlevel)
	if err == nil {
		level = l
	} else {
		level = logging.INFO
	}
}

func Logger(module string) *logging.Logger {
	lgr := logging.MustGetLogger(module)
	return lgr
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
