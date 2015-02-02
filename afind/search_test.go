package afind

import (
	"testing"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/walkablefs"
	//	"github.com/andaru/codesearch/index"
	"golang.org/x/tools/godoc/vfs/mapfs"
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
	r.AddFileRepoMatches("filename.txt", "key1",
		map[string]string{
			"1": text,
			"2": text + "2",
		})
	eq(t, 1, len(r.Matches))
	eq(t, text, r.Matches["filename.txt"]["key1"]["1"])
	eq(t, text+"2", r.Matches["filename.txt"]["key1"]["2"])
}

type _testContext struct {
	ix     indexer
	sr     searcher
	db     KeyValueStorer
	config *Config
	ctx    context.Context
}

func getTestContext(ctx context.Context) *_testContext {
	return &_testContext{
		db:     newDb(),
		config: &Config{IndexInRepo: true, NumShards: 1},
		ctx:    ctx,
	}
}

const (
	kixKey1 = "indexKey1"
)

func searchSetupIndex(files map[string]string, t *testing.T) *_testContext {
	mockIx.reset()
	fs := walkablefs.New(mapfs.New(files))
	ctx := testSearchContext(fs)
	test := getTestContext(ctx)
	query := NewIndexQuery(kixKey1)
	query.Dirs = []string{"."}
	query.Root = "/"

	test.ix = NewIndexer(test.config, test.db)
	ir, err := test.ix.Index(ctx, query)
	if err != nil {
		t.Error("indexing had unexpected error:", err)
	}
	// Index doesn't set the repo in the db, so we have to.
	test.db.Set(ir.Repo.Key, ir.Repo)

	return test
}

func TestSearch(t *testing.T) {
	files := map[string]string{
		"src/foo/foo.go":      "package foo\n",
		"README":              "Root directory README file\n\n",
		"src/hasbar/bar1.go":  "has foo and bar\nwith bar and foo also\n",
		"src/nobar/nobar1.go": "there's nothing but foo here",
	}
	test := searchSetupIndex(files, t)
	test.sr = NewSearcher(test.config, test.db)

	// Should produce an error due to no repokeys
	query := NewSearchQuery("^foo", "", true, []string{})
	sr, err := test.sr.Search(test.ctx, query)
	if sr.Error != "SearchQuery must have a non-empty RepoKeys" {
		t.Errorf("expected missing RepoKeys error, got '%s'", sr.Error)
	}

	// Should work, but not match anything
	query = NewSearchQuery("^foo", "", true, []string{kixKey1})
	sr, err = test.sr.Search(test.ctx, query)
	if err != nil {
		t.Error("search unexpected error:", err)
	}
	if sr.NumMatches != 0 {
		t.Error("want 0 matches, got", sr.NumMatches)
	}
	if sr.Error != "" {
		t.Error("want no error, got", sr.Error)
	}
	if sr.Errors[kixKey1] != nil {
		t.Errorf("want no repo error for %v, got %v",
			kixKey1, sr.Errors[kixKey1])
	}

	// Should match things now
	query = NewSearchQuery("foo", "", true, []string{kixKey1})
	sr, err = test.sr.Search(test.ctx, query)
	if err != nil {
		t.Error("search unexpected error:", err)
	}
	// Should get 4 matches (lines)
	if sr.NumMatches != 4 {
		t.Error("want 4 matches, got", sr.NumMatches)
	}
	// ... in 3 files
	if len(sr.Matches) != 3 {
		t.Error("want 3 files matched, got", len(sr.Matches))
	}
}
