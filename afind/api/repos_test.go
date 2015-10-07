package api

import (
	"testing"

	"github.com/andaru/afind/afind"
)

func TestReposGet(t *testing.T) {
	sys := newRpcServer(t, getTestConfig())
	addr := sys.rpcServer.l.Addr().String()
	defer sys.rpcServer.CloseNoErr()

	// Add a repo
	repo1 := afind.NewRepo()
	repo1.Key = "repo1"
	repo1.IndexPath = "/"
	repo1.Root = "/"
	repo1.NumFiles = 2
	repo1.NumShards = 1

	testAddRepos(sys, map[string]*afind.Repo{
		"repo1": repo1,
	})

	cl, err := NewRpcClient(addr)
	if err != nil {
		t.Error("unexpected client error:", err)
	}
	repos := NewReposClient(cl)

	// check for non-existance case; no repos
	r, err := repos.Get("not here")
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if len(r) != 0 {
		t.Error("want no repos, got", len(r))
	}

	r, err = repos.Get("repo1")
	if len(r) != 1 {
		t.Error("want 1 repo, got", len(r))
	}

	// Add another repo, and try GetAll also
	repo2 := afind.NewRepo()
	repo2.Key = "repo2"
	repo2.IndexPath = "/"
	repo2.Root = "/"
	repo2.NumFiles = 4
	repo2.NumShards = 2

	testAddRepos(sys, map[string]*afind.Repo{
		"repo2": repo2,
	})

	r, err = repos.Get("repo1")
	if len(r) != 1 {
		t.Error("want 1 repo, got", len(r))
	}
	r, err = repos.Get("repo2")
	if len(r) != 1 {
		t.Error("want 1 repo, got", len(r))
	}
	r, err = repos.GetAll()
	if len(r) != 2 {
		t.Error("want 2 repos, got", len(r))
	}
	if _, ok1 := r["repo1"]; !ok1 {
		t.Error("didn't find expected 'repo1' key")
	}
	if _, ok2 := r["repo2"]; !ok2 {
		t.Error("didn't find expected 'repo2' key")
	}

	// Test deletion
	_ = repos.Delete("repo1")
	r, err = repos.Get("repo1")
	if len(r) != 0 {
		t.Error("want 0 repo, got", len(r))
	}
	repos.Delete("repo2")
	r, err = repos.Get("repo2")
	if len(r) != 0 {
		t.Error("want 0 repo, got", len(r))
	}
	r, err = repos.GetAll()
	if len(r) != 0 {
		t.Error("want 0 repos, got", len(r))
	}

}
