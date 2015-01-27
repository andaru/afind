package api

import (
	"testing"

	"github.com/andaru/afind/afind"
)

func setRepos(repos afind.KeyValueStorer) {
	setrepo := func(r *afind.Repo) {
		repos.Set(r.Key, r)
	}
	repo1 := afind.NewRepo()
	repo1.Key = "1"
	repo1.Meta["foo1"] = "bar"
	repo1.Meta["foo2"] = "barbazfoo"

	repo2 := afind.NewRepo()
	repo2.Key = "2"
	repo2.Meta["foo1"] = "bar2"
	repo2.Meta["foo2"] = "bazbar"

	setrepo(repo1)
	setrepo(repo2)
}

func TestGetReposKeys(t *testing.T) {
	repos := afind.NewDb()
	setRepos(repos)

	request1 := afind.SearchQuery{}
	// should get all keys
	actual0 := getRepos(repos, request1)
	if len(actual0) != repos.Size() {
		t.Error("want", repos.Size(), "repos, got", len(actual0))
	}
	// now select just one repo key
	request1.RepoKeys = []string{"1"}
	actual1 := getRepos(repos, request1)
	if len(actual1) != 1 {
		t.Error("want 1 repo, got", len(actual1))
	} else if actual1[0].Key != "1" {
		t.Error("expected repo key 1, got", actual1[0].Key)
	}
	// select both (all) keys
	request1.RepoKeys = []string{"1", "2"}
	actual2 := getRepos(repos, request1)
	if len(actual2) != 2 {
		t.Error("want 2 repo, got", len(actual2))
	}
}

func TestGetReposMeta(t *testing.T) {
	repos := afind.NewDb()
	setRepos(repos)

	request1 := afind.SearchQuery{}
	request1.Meta = make(afind.Meta)

	request1.Meta["foo1"] = "not there"
	actual0 := getRepos(repos, request1)
	if len(actual0) != 0 {
		t.Error("want 0 repos, got", len(actual0))
	}

	request1.Meta["foo1"] = "bar"
	actual1 := getRepos(repos, request1)
	if len(actual1) != 1 {
		t.Error("want 1 repo, got", len(actual1))
	} else if actual1[0].Key != "1" {
		t.Error("expected repo key 1, got", actual1[0].Key)
	}
}

func TestGetReposMetaRegexp(t *testing.T) {
	repos := afind.NewDb()
	setRepos(repos)

	request := afind.SearchQuery{}
	request.Meta = make(afind.Meta)
	// Use regular expression matches
	request.MetaRegexpMatch = true

	// Explicitly match nothing
	request.Meta["foo1"] = "!.*"
	actual0 := getRepos(repos, request)
	if len(actual0) != 0 {
		t.Error("want 0 repos, got", len(actual0))
	}

	// Now match just one repo, each at a time, using
	// various matches
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo1"] = "^.ar$"
	actual1 := getRepos(repos, request)
	if len(actual1) != 1 {
		t.Error("want 1 repo, got", len(actual1))
	} else if actual1[0].Key != "1" {
		t.Error("expected repo key 1, got", actual1[0].Key)
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo2"] = "barbaz"
	actual2 := getRepos(repos, request)
	if len(actual2) != 1 {
		t.Error("want 1 repo, got", len(actual2))
	} else if actual2[0].Key != "1" {
		t.Error("expected repo key 1, got", actual2[0].Key)
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo2"] = "^baz"
	actual3 := getRepos(repos, request)
	if len(actual3) != 1 {
		t.Error("want 1 repo, got", len(actual3))
	} else if actual3[0].Key != "2" {
		t.Error("expected repo key 2, got", actual3[0].Key)
	}

	// Match multiple repos
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo1"] = ".*"
	actual4 := getRepos(repos, request)
	if len(actual4) != 2 {
		t.Error("want 2 repos, got", len(actual4))
	}

	// Partial match, should match both repos (bar unanchored is
	// in both repo metadata for key foo2)
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo2"] = "bar"
	actual5 := getRepos(repos, request)
	if len(actual5) != 2 {
		t.Error("want 2 repos, got", len(actual5))
	}

	// Negative regexps
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo1"] = "!.*"
	actual6 := getRepos(repos, request)
	if len(actual6) != 0 {
		t.Error("want 0 repos, got", len(actual6))
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo1"] = "!notthere"
	actual7 := getRepos(repos, request)
	if len(actual7) != 2 {
		t.Error("want 2 repos, got", len(actual7))
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	// This should match nothing, since it's really "not anything"
	request.Meta["foo1"] = "!"
	actual8 := getRepos(repos, request)
	if len(actual8) != 0 {
		t.Error("want 0 repos, got", len(actual8))
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	// This should match only the first repo
	request.Meta["foo1"] = "!bar2"
	actual9 := getRepos(repos, request)
	if len(actual9) != 1 {
		t.Error("want 1 repo, got", len(actual9))
	} else if actual9[0].Key != "1" {
		t.Error("expected repo key 1, got", actual9[0].Key)
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	// This should match everything, since it's really "not nothing"
	request.Meta["foo1"] = "!^$"
	actual10 := getRepos(repos, request)
	if len(actual10) != 2 {
		t.Error("want 2 repos, got", len(actual10))
	}
}

func TestGetReposEmptyOtherKeys(t *testing.T) {
	// Test empty matches, both normal and regexp
	repos := afind.NewDb()
	setRepos(repos)

	request := afind.SearchQuery{}
	request.Meta = make(afind.Meta)

	// Use non-regular expression matches (default) first
	request.MetaRegexpMatch = false
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	// Empty match string matches only empty metadata values
	request.Meta["foo2"] = ""
	actual0 := getRepos(repos, request)
	if len(actual0) != 0 {
		t.Error("want 0 repos, got", len(actual0))
	}

	// Use regular expression matches
	request.MetaRegexpMatch = true
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
}
