package afind

import (
	"net"
	"net/rpc"
	"os"
	"strings"

	"path"
	"testing"
)

// Test outside of the RPC framework
func TestRpcIndexFunction(t *testing.T) {
	key := "key"
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	ir := newIndexRequest(key, path.Join(cwd, "./testdata/repo1/"), []string{"."})

	repos := newDb()
	svc := NewService(repos)
	rpcsvc := newRpcService(svc)
	resp := newIndexResponse()
	err = rpcsvc.Index(ir, resp)

	if err != nil {
		t.Error("unexpected error:", err)
	}

	// there was one dir, so only use one shard
	if resp.Repo.SizeData < 1 {
		t.Error("got zero size data")
	}
	if resp.Repo.SizeIndex < 1 {
		t.Error("got zero size index")
	}
	if resp.Repo.NumDirs != 3 {
		t.Error("got", resp.Repo.NumDirs, "dirs, want 3")
	}
	if resp.Repo.NumFiles != 3 {
		t.Error("got", resp.Repo.NumFiles, "files, want 3")
	}
}

// Test using an RPC server
func TestRpcIndexWithServer(t *testing.T) {
	repos := newDb()
	svc := NewService(repos)
	rpcsvc := newRpcService(svc)
	svr := rpc.NewServer()
	svr.RegisterName("Afind", rpcsvc)
	l, e := net.Listen("tcp", ":56789")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go svr.Accept(l)

	// Now connect as the client
	client, err := rpc.Dial("tcp", "localhost:56789")
	if err != nil {
		log.Fatal("dialing:", err)
	}
	key := "key"
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	args := newIndexRequest(key, path.Join(cwd, "./testdata/repo1/"), []string{"."})
	reply := IndexResponse{}

	err = client.Call("Afind.Index", args, &reply)
	if err != nil {
		t.Error("unexpected error:", err)
	}
}

// Test Index and GetRepo (includes SetRepo calls from Index)
func TestGetRepo(t *testing.T) {
	rs := newDb()
	svc := NewService(rs)
	rpcsvc := newRpcService(svc)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	ir := newIndexRequest("key1", path.Join(cwd, "./testdata/repo1/"),
		[]string{"dir1"})
	ir2 := newIndexRequest("key2", path.Join(cwd, "./testdata/repo1/"),
		[]string{"dir2"})

	indexresp := newIndexResponse()
	indexresp2 := newIndexResponse()
	err = rpcsvc.Index(ir, indexresp)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	err = rpcsvc.Index(ir2, indexresp2)
	if err != nil {
		t.Error("unexpected error:", err)
	}

	seen := make(map[string]bool)
	var repos Repos
	keys := []string{"key1", "key2"}
	err = rpcsvc.GetRepos(keys, &repos)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	for k, v := range repos.Repos {
		if !strings.Contains(v.IndexPath, "/repo1/key") {
			t.Error("want '/repo1/key' in v.IndexPath which is",
				v.IndexPath)
		} else {
			seen[k] = true
		}
	}
	if len(seen) != 2 {
		t.Error("got", len(seen), "repos, want 2")
	}
}

func TestReindexFailure(t *testing.T) {
	rs := newDb()
	svc := NewService(rs)
	rpcsvc := newRpcService(svc)

	key := "SAME KEY"
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	ir := newIndexRequest(key, path.Join(cwd, "./testdata/repo1/"),
		[]string{"dir1"})
	// even with different data, we can't index this again unless
	// the first one failed.
	ir2 := newIndexRequest(key, path.Join(cwd, "./testdata/repo1/"),
		[]string{"dir2"})
	resp := newIndexResponse()
	resp2 := newIndexResponse()
	err = rpcsvc.Index(ir, resp)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	err = rpcsvc.Index(ir2, resp2)
	if err == nil {
		t.Error("expected an error")
	}
	if !strings.Contains(err.Error(), "Cannot replace existing") {
		t.Error("error message [", err.Error(), "] was unexpected")
	}
}

func TestRpcSearch(t *testing.T) {
	rs := newDb()
	svc := NewService(rs)
	rpcsvc := newRpcService(svc)
	key := "index1"
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	ir := newIndexRequest(key, path.Join(cwd, "./testdata/repo1/"),
		[]string{"dir1"})
	iresp := newIndexResponse()

	err = rpcsvc.Index(ir, iresp)
	if err != nil {
		t.Error("unexpected error:", err)
		return
	}
	sreq := newSearchRequest("(dir1|dir2)", "", false, []string{})
	sresp := newSearchResponse()
	err = rpcsvc.Search(sreq, sresp)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if sresp.NumLinesMatched != 2 {
		t.Error("expected 1 line match, got", sresp.NumLinesMatched)
	}
}

// Test Index and GetRepo (includes SetRepo calls from Index)
func TestGetAllRepos(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	svc := NewService(newDb())
	rpcsvc := newRpcService(svc)

	ir := newIndexRequest("key1", path.Join(cwd, "./testdata/repo1/"),
		[]string{"dir1"})
	ir2 := newIndexRequest("key2", path.Join(cwd, "./testdata/repo1/"),
		[]string{"dir2"})
	ir3 := newIndexRequest("key3", path.Join(cwd, "./testdata/repo1/"),
		[]string{"."})

	indexresp := newIndexResponse()
	indexresp2 := newIndexResponse()
	indexresp3 := newIndexResponse()
	err = rpcsvc.Index(ir, indexresp)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	err = rpcsvc.Index(ir2, indexresp2)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	err = rpcsvc.Index(ir3, indexresp3)
	if err != nil {
		t.Error("unexpected error:", err)
	}

	var repos map[string]*Repo
	err = rpcsvc.GetAllRepos(true, &repos)
	if len(repos) != 3 {
		t.Error("got", len(repos), "repos, want 3")
	}
	seen := make(map[string]bool)
	for k, v := range repos {
		if !strings.Contains(v.IndexPath, "/repo1/key") {
			t.Error("want '/repo1/key' in v.IndexPath which is",
				v.IndexPath)
		} else {
			seen[k] = true
		}
	}
	if len(seen) != 3 {
		t.Error("got", len(seen), "repos, want 3")
	}
}

// /// Test indexing via the actual rpc server
// func TestGetPrefixRepos(t *testing.T) {
// 	addr := ":30303"
// 	rs := newDb()
// 	svc := NewService(rs)
// 	rpcsvc := newRpcService(svc)
// 	svr := rpc.NewServer()
// 	svr.RegisterName("Afind", rpcsvc)
// 	l, e := net.Listen("tcp", addr)
// 	if e != nil {
// 		log.Fatal("listen error:", e)
// 	}
// 	go svr.Accept(l)

// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	ir := newIndexRequest("key1",
// 		path.Join(cwd, "./testdata/repo1/"), []string{"dir1"})
// 	ir2 := newIndexRequest("key2",
// 		path.Join(cwd, "./testdata/repo1/"), []string{"dir2"})
// 	ir3 := newIndexRequest("key3",
// 		path.Join(cwd, "./testdata/repo1/"), []string{"."})

// 	client, cerr := NewRpcClient(addr)
// 	if cerr != nil {
// 		t.Fatal(cerr)
// 	}

// 	client.Index(ir)
// 	client.Index(ir2)
// 	client.Index(ir3)

// 	length := len(repos.Repos)
// 	size := rs.Size()

// 	if length != size {
// 		t.Error(length, "!=", size)
// 	}

// 	if len(repos.Repos) != 3 {
// 		t.Error("got", len(repos.Repos), "repos, want 3")
// 	}

// 	seen := make(map[string]bool)
// 	for k, v := range repos.Repos {
// 		if !strings.HasPrefix(v.IndexPath, "key") {
// 			t.Error("want 'key' at prefix to v.IndexPath which is",
// 				v.IndexPath)
// 		} else {
// 			seen[k] = true
// 		}
// 	}
// 	if len(seen) != 3 {
// 		t.Error("got", len(seen), "repos, want 2")
// 	}
// }
