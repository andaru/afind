package afind

import (
	"testing"
)

func TestSearchQuery(t *testing.T) {
	q := NewSearchQuery(
		"search regexp", "path regexp", false, []string{"key1", "key2"})
	eq(t, "key1", q.firstKey())
	eq(t, "key2", q.RepoKeys[1])
	nokeys := NewSearchQuery("search", "path", false, []string{})
	eq(t, "", nokeys.firstKey())
}

func TestSearchResult(t *testing.T) {
	r := NewSearchResult()
	// these will panic with an uninit map error if
	// the search result is not properly initialized
	text := "// Copyright..."
	r.addFileRepoMatches("filename.txt", "key1",
		map[string]string{
			"1": text,
			"2": text + "2",
		})
	eq(t, 1, len(r.Matches))
	eq(t, text, r.Matches["filename.txt"]["key1"]["1"])
	eq(t, text+"2", r.Matches["filename.txt"]["key1"]["2"])
}
