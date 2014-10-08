package afind

import (
	"os"
	"path"
	"testing"
)

var (
	repo1 = map[string]string{
		`./dir1/file1`: `A file in dir1`,
		`./dir2/file1`: `A file in dir2`,
		`./root_file`:  `abc\nA file in root`,
	}
)

func init() {
	config.IndexInRepo = true
	config.SetNoIndex(`.afindex$`)
}

func createRepo(t *testing.T,
	svc *Service, files map[string]string, key string, paths []string) string {

	dir, err := getTempDir(key)
	if err != nil {
		t.Fatal(err)
	}
	ir := newIndexRequest(key, dir, paths)

	// Add the files to the repo
	for name, contents := range files {
		fn := path.Join(dir, name)
		dirname := path.Dir(fn)

		if merr := os.MkdirAll(dirname, 0755); merr != nil && !os.IsExist(merr) {
			t.Fatal(merr)
		}
		if f, ferr := os.Create(fn); ferr == nil {
			f.WriteString(contents)
			f.Close()
		}
	}

	_, err = svc.Indexer.Index(ir)
	if err != nil {
		t.Fatal("Expected no error during createRepo, got:", err)
	}
	return dir
}

func TestSearchRepoBothDirs(t *testing.T) {
	key := "TestSearchRepoBothDirs"
	repos := newDb()
	svc := NewService(repos)

	defer os.RemoveAll(createRepo(t, svc, repo1, key, []string{"dir1", "dir2"}))

	// Now search for things in both dirs
	sr := newSearchRequest("(dir1|dir2)", "", false, []string{key})
	resp, err := svc.Searcher.Search(sr)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if len(resp.Files) != 2 {
		t.Error("got", len(resp.Files), "file matches, want 2")
	}

	// Confirm we got the files we expected
	want := []string{"/dir1/file1", "/dir2/file1"}
	got := make([]string, 0)
	for k, _ := range resp.Files {
		for _, wantk := range want {
			if k == wantk {
				got = append(got, k)
			}
		}
	}
	if len(got) != 2 {
		t.Error("got", resp.Files, "paths, want substrings ", want)
	}
}

func TestSearchRepoEachDir(t *testing.T) {
	key := "TestSearchRepoEachDir"
	repos := newDb()
	svc := NewService(repos)
	defer os.RemoveAll(createRepo(t, svc, repo1, key, []string{"dir1", "dir2"}))

	// Now search for things in just one dir
	sr := newSearchRequest("file in dir1", "", false, []string{key})
	resp, err := svc.Searcher.Search(sr)
	if err != nil {
		t.Error("unexpected error:", err)
	}

	// Try the other dir
	sr = newSearchRequest("file in dir2", "", false, []string{key})
	resp, err = svc.Searcher.Search(sr)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if len(resp.Files) != 1 {
		t.Error("got", len(resp.Files), "file matches, want 1")
	}
}

func TestSearchWithPathRe(t *testing.T) {
	key := "TestSearchWithPathRe"
	repos := newDb()
	svc := NewService(repos)
	defer os.RemoveAll(createRepo(t, svc, repo1, key, []string{"dir1", "dir2"}))

	// Search for something that exists, but not in this dir
	sr := newSearchRequest("file in dir1", "dir2/.*", false, []string{key})
	resp, err := svc.Searcher.Search(sr)

	if err != nil {
		t.Error("unexpected error:", err)
	}
	if len(resp.Files) != 0 {
		t.Error("got", len(resp.Files), "file matches, want 0")
	}

	// Now use a pathRe which matches the path with the string 'file in dir1'
	sr = newSearchRequest("file in dir1", "dir1/.*", false, []string{key})
	resp, err = svc.Searcher.Search(sr)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if len(resp.Files) != 1 {
		t.Error("got", len(resp.Files), "file matches, want 1")
	}

	// Test that the other similar condition also matches
	sr = newSearchRequest("file in dir2", "dir2/.*", false, []string{key})
	resp, _ = svc.Searcher.Search(sr)
	if len(resp.Files) != 1 {
		t.Error("got", len(resp.Files), "file matches, want 1")
	}
}
