package utils

import (
	"os"

	"github.com/op/go-logging"
)

var (
	loglevbe logging.LeveledBackend
)

func LoggerForModuleVerbose(module string) *logging.Logger {
	return LoggerForModule(module, logging.DEBUG)
}

func LoggerForModule(module string, level logging.Level) *logging.Logger {
	logr := logging.MustGetLogger(module)
	loglevbe.SetLevel(level, module)
	return logr
}

func init() {
	setupLogging()
}

func setupLogging() {
	loglevbe = logging.AddModuleLevel(logging.NewLogBackend(os.Stderr, "", 0))
	format := logging.MustStringFormatter(
		"%{color:bold}%{level:.1s}%{time:0102 15:04:05.999999} " +
			"%{pid} %{shortfunc} %{shortfile}]%{color:reset} %{message}")
	logging.SetFormatter(format)
	logging.SetBackend(loglevbe)
}
