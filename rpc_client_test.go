package afind

import (
	"net"
	"net/rpc"
	"os"
	"path"
	"testing"
)

func newServer(t *testing.T) net.Listener {
	repos := newDb()
	svc := newService(repos)
	rpcsvc := newRpcService(svc)
	svr := rpc.NewServer()
	svr.RegisterName("Afind", rpcsvc)
	listener, e := net.Listen("tcp", ":")
	if e != nil {
		t.Fatal("listen error:", e)
	}
	go svr.Accept(listener)
	return listener
}

func closeServer(t *testing.T, listener net.Listener) {
	err := listener.Close()
	if err != nil {
		t.Log(err)
	}
}

func TestRpcClientIndex(t *testing.T) {
	server := newServer(t)
	client, err := NewRpcClient(server.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	req := IndexRequest{
		Key:  "abc",
		Root: path.Join(cwd, "./testdata/repo1"),
		Dirs: []string{"."},
	}

	reply, err := client.Index(req)
	if err != nil {
		t.Error(err)
	}
	if reply.Repo.SizeData < 1 {
		t.Error("got bad size of data", reply.Repo.SizeData)
	}
	if reply.Repo.SizeIndex < 1 {
		t.Error("got bad size of index", reply.Repo.SizeIndex)
	}
	if reply.Repo.NumDirs != 3 {
		t.Error("got", reply.Repo.NumDirs, "dirs, want 3")
	}
	if reply.Repo.NumFiles != 3 {
		t.Error("got", reply.Repo.NumFiles, "dirs, want 3")
	}
	if reply.Repo.State != OK {
		t.Error("got bad repo state: ",
			string(reply.Repo.State.String()), ", want OK")
	}
}

func TestRpcClientSearchRepo(t *testing.T) {
	newServer(t)
}

func TestRpcClientGetRepo(t *testing.T) {
	newServer(t)
}

func TestRpcClientGetAllRepos(t *testing.T) {
	newServer(t)
}
