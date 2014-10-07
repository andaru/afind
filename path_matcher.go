package afind

import (
	"github.com/kellydunn/go-art"
	"strings"
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
		if v := e.extensions.Search([]byte(extension)); v != nil {
			return true
		}
	}
	return false
}

func (e PathMatcher) MatchFile(name string) bool {
	return (e.matchExtension(name) ||
		strings.HasSuffix(name, "#"))
}
