package afind

import (
	"path/filepath"
	"runtime"
)

// Returns the calling function's name
func FN() string {
	return FNn(2)
}

// Returns the function N stack frames from here's name
func FNn(n int) string {
	pc, _, _, _ := runtime.Caller(n)
	return runtime.FuncForPC(pc).Name()
}

func normalizeUri(uri string) (string, error) {
	// todo: handle schemes, these are local filesystem paths only
	abspath, _ := filepath.Abs(uri)
	return abspath, nil
}
