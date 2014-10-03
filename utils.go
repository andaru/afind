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

func repoIndexable(key string, repos KeyValueStorer) bool {
	value := repos.Get(key)
	log.Debug("repoIndexable key=%v value=%v", key, value)
	if value != nil {
		return value.(*Repo).State < ERROR
	}
	return true
}
