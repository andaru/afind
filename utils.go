package afind

import (
	"path/filepath"

	"github.com/op/go-logging"
)

// the logging handle for the afind package
var (
	log = logging.MustGetLogger("afind")
)

func normalizeUri(uri string) (string, error) {
	// todo: handle schemes, these are local filesystem paths only
	abspath, _ := filepath.Abs(uri)
	return abspath, nil
}
