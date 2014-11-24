package afind

import (
	"strings"
	"time"

	"github.com/kellydunn/go-art"
)

func newPathMatcher() *PathMatcher {
	return &PathMatcher{extensions: art.NewArtTree()}
}

type PathMatcher struct {
	extensions   *art.ArtTree
	maxScanDepth int
}

func (e PathMatcher) AddExtension(extension string) {
	if extension == "" {
		return
	}
	if extension[0] == '.' {
		extension = extension[1:]
	}
	e.extensions.Insert([]byte(extension), struct{}{})
}

func (e PathMatcher) matchExtension(name string) bool {
	var extension string
	if n := strings.LastIndex(name, "."); n == -1 {
		// doesn't have a traditional extension, so
		// we cannot match it here.
		return false
	} else {
		extension = name[n+1:]
		v := e.extensions.Search([]byte(extension))
		if v != nil {
			return true
		}
	}
	return false
}

func (e PathMatcher) MatchFile(name string) bool {
	return e.matchExtension(name) || strings.HasSuffix(name, "#")
}

func (e PathMatcher) TimedMatchFile(name string) (bool, time.Duration) {
	start := time.Now()
	return e.MatchFile(name), time.Since(start)
}
