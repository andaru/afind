package afind

import (
	"testing"
)

func TestPathMatcher(t *testing.T) {
	pm := newPathMatcher()
	pm.AddExtension(".git")
	pm.AddExtension(".svn")
	pm.AddExtension(".hg")

	eq(t, true, pm.matchExtension("foo/bar/.git"))
	eq(t, false, pm.matchExtension("foo/bar/.git/foo"))
}
