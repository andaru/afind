package afind

import (
	"strings"
	"testing"
)

func createRepo(t *testing.T, key, root string, paths []string) IndexRequest {
	ir := newIndexRequest(key, root, paths)
	_, err := makeIndex(ir)
	if err != nil {
		t.Fatal("Expected no error during createRepo, got:", err)
	}
	return ir
}

func TestSearchRepoBothDirs(t *testing.T) {
	key := "TestSearchRepoBothDirs"
	ir := createRepo(t, key, "./testdata/repo1/", []string{"dir1", "dir2"})
	repo := newRepoFromIndexRequest(ir)
	repos := newDb()
	repos.Set(key, repo)
	svc := newService(repos)

	// Now search for things in both dirs
	sr := newSearchRequest("(dir1|dir2)", "", false, []string{key})
	resp, err := svc.Searcher.Search(sr)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if len(resp.Files) != 2 {
		t.Error("got ", len(resp.Files), " file matches, want 2")
	}

	// Confirm we got the files we expected
	want := []string{"/dir1/file1", "/dir2/file1"}
	got := make([]string, 0)
	for k, _ := range resp.Files {
		for _, wantk := range want {
			if strings.Contains(k, wantk) {
				got = append(got, k)
			}
		}
	}
	if len(got) != 2 {
		t.Error("got ", got, " matching paths, want substrings ", want)
	}
}

func TestSearchRepoEachDir(t *testing.T) {
	key := "TestSearchRepoEachDir"
	ir := createRepo(t, key, "./testdata/repo1/", []string{"dir1", "dir2"})
	repo := newRepoFromIndexRequest(ir)
	repos := newDb()
	repos.Set(key, repo)
	svc := newService(repos)

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
		t.Error("got ", len(resp.Files), " file matches, want 1")
	}
}

func TestSearchWithPathRe(t *testing.T) {
	key := "TestSearchWithPathRe"
	ir := createRepo(t, key, "./testdata/repo1/", []string{"dir1", "dir2"})
	repo := newRepoFromIndexRequest(ir)
	repos := newDb()
	repos.Set(key, repo)
	svc := newService(repos)

	// Search for something that exists, but not in this dir
	sr := newSearchRequest("file in dir1", ".*/dir2/.*", false, []string{key})
	resp, err := svc.Searcher.Search(sr)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if len(resp.Files) != 0 {
		t.Error("got ", len(resp.Files), " file matches, want 0")
	}

	// Now use a pathRe which matches the path with the string 'file in dir1'
	sr = newSearchRequest("file in dir1", ".*/dir1/.*", false, []string{key})
	resp, err = svc.Searcher.Search(sr)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if len(resp.Files) != 1 {
		t.Error("got ", len(resp.Files), " file matches, want 1")
	}

	// Test that the other similar condition also matches
	sr = newSearchRequest("file in dir2", ".*/dir2/.*", false, []string{key})
	resp, _ = svc.Searcher.Search(sr)
	if len(resp.Files) != 1 {
		t.Error("got ", len(resp.Files), " file matches, want 1")
	}
}
