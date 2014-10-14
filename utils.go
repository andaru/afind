package afind

import (
	"github.com/op/go-logging"
	"os"
)

// the logging handle for use throughout the afind package
var (
	log *logging.Logger
)

func init() {
	LoggerStderr()
	log = logging.MustGetLogger("afind")
}

func LoggerStderr() {
	logger := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(logger)
	logging.SetFormatter(logging.GlogFormatter)

}
