package common

import (
	"os"

	"github.com/op/go-logging"
)

func LoggerStderr() {
	logger := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(logger)
	logging.SetFormatter(logging.GlogFormatter)
}
