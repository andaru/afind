package api

import (
	"net"
	"os/user"
	"strings"
	"sync"
	"testing"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/afind"
)

func newRepo(key string) *afind.Repo {
	r := afind.NewRepo()
	r.Key = key
	r.State = afind.OK
	return r
}

func setRepos(repos afind.KeyValueStorer) {
	repo1 := newRepo("1")
	repo1.Meta["foo1"] = "bar"
	repo1.Meta["foo2"] = "barbazfoo"

	repo2 := newRepo("2")
	repo2.Meta["foo1"] = "bar2"
	repo2.Meta["foo2"] = "bazbar"

	repos.Set(repo1.Key, repo1)
	repos.Set(repo2.Key, repo2)
}

func TestGetReposKeys(t *testing.T) {
	repos := afind.NewDb()
	setRepos(repos)

	request1 := afind.SearchQuery{}
	// should get all keys
	actual0 := getRepos(repos, request1, 0)
	if len(actual0) != repos.Size() {
		t.Error("want", repos.Size(), "repos, got", len(actual0))
	}
	// now select just one repo key
	request1.RepoKeys = []string{"1"}
	actual1 := getRepos(repos, request1, 0)
	if len(actual1) != 1 {
		t.Error("want 1 repo, got", len(actual1))
	} else if actual1[0].Key != "1" {
		t.Error("expected repo key 1, got", actual1[0].Key)
	}
	// select both (all) keys
	request1.RepoKeys = []string{"1", "2"}
	actual2 := getRepos(repos, request1, 0)
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
	actual0 := getRepos(repos, request1, 0)
	if len(actual0) != 0 {
		t.Error("want 0 repos, got", len(actual0))
	}

	request1.Meta["foo1"] = "bar"
	actual1 := getRepos(repos, request1, 0)
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
	actual0 := getRepos(repos, request, 0)
	if len(actual0) != 0 {
		t.Error("want 0 repos, got", len(actual0))
	}

	// Now match just one repo, each at a time, using
	// various matches
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo1"] = "^.ar$"
	actual1 := getRepos(repos, request, 0)
	if len(actual1) != 1 {
		t.Error("want 1 repo, got", len(actual1))
	} else if actual1[0].Key != "1" {
		t.Error("expected repo key 1, got", actual1[0].Key)
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo2"] = "barbaz"
	actual2 := getRepos(repos, request, 0)
	if len(actual2) != 1 {
		t.Error("want 1 repo, got", len(actual2))
	} else if actual2[0].Key != "1" {
		t.Error("expected repo key 1, got", actual2[0].Key)
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo2"] = "^baz"
	actual3 := getRepos(repos, request, 0)
	if len(actual3) != 1 {
		t.Error("want 1 repo, got", len(actual3))
	} else if actual3[0].Key != "2" {
		t.Error("expected repo key 2, got", actual3[0].Key)
	}

	// Match multiple repos
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo1"] = ".*"
	actual4 := getRepos(repos, request, 0)
	if len(actual4) != 2 {
		t.Error("want 2 repos, got", len(actual4))
	}

	// Partial match, should match both repos (bar unanchored is
	// in both repo metadata for key foo2)
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo2"] = "bar"
	actual5 := getRepos(repos, request, 0)
	if len(actual5) != 2 {
		t.Error("want 2 repos, got", len(actual5))
	}

	// Negative regexps
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo1"] = "!.*"
	actual6 := getRepos(repos, request, 0)
	if len(actual6) != 0 {
		t.Error("want 0 repos, got", len(actual6))
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo1"] = "!notthere"
	actual7 := getRepos(repos, request, 0)
	if len(actual7) != 2 {
		t.Error("want 2 repos, got", len(actual7))
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	request.Meta["foo2"] = "!barbaz"
	actual8 := getRepos(repos, request, 0)
	if len(actual8) != 1 {
		t.Error("want 1 repo, got", len(actual8))
	}
	if actual8[0].Key != "2" {
		t.Error("want repo key 2, got", actual8[0].Key)
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	// This should match nothing, since it's really "not anything"
	request.Meta["foo1"] = "!"
	actual9 := getRepos(repos, request, 0)
	if len(actual9) != 0 {
		t.Error("want 0 repos, got", len(actual9))
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	// This should match only the first repo
	request.Meta["foo1"] = "!bar2"
	actual10 := getRepos(repos, request, 0)
	if len(actual10) != 1 {
		t.Error("want 1 repo, got", len(actual10))
	} else if actual10[0].Key != "1" {
		t.Error("expected repo key 1, got", actual10[0].Key)
	}

	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	// This should match everything, since it's really "not nothing"
	request.Meta["foo1"] = "!^$"
	actual11 := getRepos(repos, request, 0)
	if len(actual11) != 2 {
		t.Error("want 2 repos, got", len(actual11))
	}
}

func TestGetReposEmptyOtherKeys(t *testing.T) {
	// Test empty matches, both normal and regexp
	repos := afind.NewDb()
	setRepos(repos)

	request := afind.SearchQuery{}
	request.Meta = make(afind.Meta)

	request.MetaRegexpMatch = false
	delete(request.Meta, "foo1")
	delete(request.Meta, "foo2")
	// Empty match string matches only empty metadata values
	request.Meta["foo2"] = ""
	actual0 := getRepos(repos, request, 0)
	if len(actual0) != 0 {
		t.Error("want 0 repos, got", len(actual0))
	}
}

type testSystem struct {
	repos     afind.KeyValueStorer
	indexer   testIndexer
	searcher  testSearcher
	config    afind.Config
	server    *baseServer
	rpcServer *RpcServer
}

var (
	t_default_fe_port = "57050"
	t_rpc_port        = "57071"
	t_http_port       = "57072"
	t_fe_host         = "127.0.0.2"
	t_be_host         = "127.0.0.202"
)

func getTestConfig() (c afind.Config) {
	c = afind.Config{RepoMeta: afind.Meta{}}
	c.RepoMeta.SetHost("testhost")
	c.RpcBind = "127.0.0.99:" + t_default_fe_port
	c.IndexInRepo = true
	c.NumShards = 1
	c.MaxSearchC = 8
	return c
}

func getTestConfigHostPort(host, port string) (c afind.Config) {
	c = getTestConfig()
	c.RepoMeta.SetHost(host)
	c.RpcBind = host + ":" + port
	c.RepoMeta["port.rpc"] = port
	return c
}

type testSearcher struct {
	called map[string]int
	lock   *sync.Mutex
}

func newTestSearcher() testSearcher {
	return testSearcher{map[string]int{}, &sync.Mutex{}}
}

var (
	ktSearchQueries = map[string]*afind.SearchResult{
		"repo1_empty": afind.NewSearchResult(),
		"repo1_foo":   afind.NewSearchResult(),
		"remote1_foo": afind.NewSearchResult(),
	}
)

func (i testSearcher) Search(
	ctx context.Context,
	query afind.SearchQuery) (sr *afind.SearchResult, err error) {

	i.called["Search"]++
	// match the request against our list of responses to return
	key := query.RepoKeys[0] + "_" + query.Re
	sr = ktSearchQueries[key]
	if sr == nil {
		panic("unknown result for key: " + key)
	}
	return
}

func newTestAfind(c afind.Config) (sys testSystem) {
	sys = testSystem{
		repos:    afind.NewDb(),
		indexer:  newTestIndexer(),
		searcher: newTestSearcher(),
		config:   c,
	}
	return
}

func newRpcServer(t *testing.T, c afind.Config) testSystem {
	sys := newTestAfind(c)
	server := NewServer(sys.repos, sys.indexer, sys.searcher, &c)
	sys.server = server
	rpcListener, err := sys.config.ListenerRpc()
	if err != nil {
		t.Fatal("rpcListen unexpected:", err)
	}
	rpcServer := NewRpcServer(rpcListener, server)
	sys.rpcServer = rpcServer
	rpcServer.Register()
	go func() {
		if e := rpcServer.Serve(); e != nil {
			t.Log("rpcServer.Serve() returned error:", e)
		}
	}()
	return sys
}

func TestListenerBindError(t *testing.T) {
	// make the config un serverable by giving a bad RPC server port
	// posix non-portable (uid 0)?
	user, err := user.Current()
	if user.Uid == "0" {
		t.SkipNow()
	}
	sys := newTestAfind(getTestConfig())
	sys.config.RpcBind = ":1"
	// In theory, this may still block if port 1 can be bound, so
	// we have to guard with a brief timeout.
	done := make(chan struct{}, 1)
	go func() {
		_, err = sys.config.ListenerRpc()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Millisecond):
		// if we blocked, just skip this test
		t.SkipNow()
	}

	if err == nil {
		t.Error("expected an RPC bind error")
	}
	if nerr, ok := err.(net.Error); !ok {
		t.Error("expected an RPC bind error")
	} else if !strings.Contains(nerr.Error(), "permission denied") {
		t.Error("expected an RPC bind error")
	}
}

func TestSearchAgainstEmptyAfindDb(t *testing.T) {
	sys := newRpcServer(t, getTestConfig())
	defer sys.rpcServer.CloseNoErr()

	cl, err := NewRpcClient(sys.config.RpcBind)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	afindd := NewSearcherClient(cl)
	defer afindd.Close()

	query := afind.NewSearchQuery("foo", "", true, []string{})
	sr, err := afindd.Search(context.Background(), query)
	// There's nothing in the index yet, but we should get no error.
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if sr.NumMatches != 0 {
		t.Error("want 0 matches, got", sr.NumMatches)
	}
	if sr.Durations.PostingQuery != 0 {
		t.Error("want 0 posting query time, got", sr.Durations.PostingQuery)
	}
	if sr.Durations.Search == 0 {
		t.Error("want a non-zero wallclock search time, got 0")
	}
}

func testAddRepos(sys testSystem, repos map[string]*afind.Repo) {
	for k, v := range repos {
		sys.repos.Set(k, v)
	}
}

func TestSearchSingleRepo(t *testing.T) {
	sys := newRpcServer(t, getTestConfig())
	defer sys.rpcServer.CloseNoErr()

	repo := newRepo("repo1")
	repo.Root = "/"
	repo.NumShards = 1
	testAddRepos(sys, map[string]*afind.Repo{"repo1": repo})

	cl, err := NewRpcClient(sys.config.RpcBind)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	afindd := NewSearcherClient(cl)
	defer afindd.Close()

	query := afind.NewSearchQuery("empty", "", true, []string{})
	sr, err := afindd.Search(context.Background(), query)
	// There's nothing in the index yet, but we should get no error.
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if sr.NumMatches != 0 {
		t.Error("want 0 matches, got", sr.NumMatches)
	}
	if sr.Durations.PostingQuery != 0 {
		t.Error("want 0 posting query time, got", sr.Durations.PostingQuery)
	}
	if sr.Durations.Search == 0 {
		t.Error("want a non-zero wallclock search time, got 0")
	}

	// Now try a real search
	result := ktSearchQueries["repo1_foo"]
	result.AddFileRepoMatches(
		"src/foo.cpp", "repo1", map[string]string{"1": "foo bar baz"})
	result.Durations.PostingQuery = time.Duration(180)
	result.Durations.Search = time.Duration(1000)
	query = afind.NewSearchQuery("foo", "", true, []string{"repo1"})
	sr, err = afindd.Search(context.Background(), query)

	if sr.Durations.CombinedPostingQuery != 180 {
		t.Error("want CombinedPostingQuery of 180, got", sr.Durations.CombinedPostingQuery)
	}
	if sr.Durations.CombinedSearch != 1000 {
		t.Error("want CombinedSearch of 1us, got", sr.Durations.CombinedSearch)
	}
	if sr.Durations.Search == 0 {
		t.Error("want non-zero Search duration")
	}
	if len(sr.Matches) != 1 {
		t.Error("want 1 match, got", len(sr.Matches))
	}
	if _, ok := sr.Matches["src/foo.cpp"]; !ok {
		t.Error("did not get expected file src/foo.cpp")
	}
	if _, ok := sr.Matches["src/foo.cpp"]["repo1"]; !ok {
		t.Error("did not get expected repo, repo1")
	}
	if _, ok := sr.Matches["src/foo.cpp"]["repo1"]["1"]; !ok {
		t.Error("did not get expected line number 1")
	}

}

func TestRemoteSearch(t *testing.T) {
	// In this test, we setup two servers and perform a query
	// to the first which will have to proxy the query to the
	// second. Both servers run on localhost using two different
	// addresses in 127.0.0.0/8
	fe := newRpcServer(t,
		getTestConfigHostPort(t_fe_host, t_rpc_port))
	be := newRpcServer(t,
		getTestConfigHostPort(t_be_host, t_rpc_port))
	defer fe.rpcServer.CloseNoErr()
	defer fe.rpcServer.CloseNoErr()

	// Add the remote repo, which lives on the backend host
	repo := newRepo("remote1")
	repo.Root = "/"
	repo.NumShards = 1
	repo.Meta.SetHost(t_be_host)
	testAddRepos(fe, map[string]*afind.Repo{"remote1": repo})
	testAddRepos(be, map[string]*afind.Repo{"remote1": repo})

	cl, err := NewRpcClient(fe.config.RpcBind)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	afindd := NewSearcherClient(cl)
	defer afindd.Close()

	result := ktSearchQueries["remote1_foo"]
	result.AddFileRepoMatches(
		"foo/bar/test.txt", "remote1",
		map[string]string{"100": "excellent, foo, excellent"})
	result.Durations.PostingQuery = time.Duration(180)
	result.Durations.Search = time.Duration(1000)
	result.NumMatches = 1
	result.Repos = map[string]*afind.Repo{"remote1": repo}
	ktSearchQueries["remote1_foo"] = result

	query := afind.NewSearchQuery("foo", "", true, []string{})
	query.Meta.SetHost(t_be_host)

	sr, err := afindd.Search(context.Background(), query)
	// There's nothing in the index yet, but we should get no error.
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if sr.NumMatches != 1 {
		t.Error("want 1 match, got", sr.NumMatches)
	}
	if len(sr.Matches) != 1 {
		t.Error("want 1 file matched, got", len(sr.Matches))
	} else if _, ok := sr.Matches["foo/bar/test.txt"]; !ok {
		t.Error("want match with path foo/bar/test.txt")
	} else if _, ok := sr.Matches["foo/bar/test.txt"]["remote1"]; !ok {
		t.Error("want match in remote1 repo")
	} else if v, ok := sr.Matches["foo/bar/test.txt"]["remote1"]["100"]; !ok {
		t.Error("want match on line 100")
	} else if v != "excellent, foo, excellent" {
		t.Error("wanted match string 'excellent, foo, excellent', got", v)
	}
	if sr.Durations.Search == 0 {
		t.Error("want non-zero search duration")
	}
}
