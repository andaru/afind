package afind

import (
	"runtime"
)

// Returns the calling function's name
func FN() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}


















