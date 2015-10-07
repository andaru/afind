package api

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/afind"
)

type testIndexer struct {
	called map[string]int
	lock   *sync.Mutex
}

func newTestIndexer() testIndexer {
	return testIndexer{map[string]int{}, &sync.Mutex{}}
}

func (i testIndexer) Index(
	ctx context.Context,
	req afind.IndexQuery) (ir *afind.IndexResult, err error) {

	i.called["Index"]++
	ir = afind.NewIndexResult()
	ir.Repo = ktIndexRepo1
	return
}

var (
	ktIndexRepo1 = &afind.Repo{
		Key:             "key",
		IndexPath:       "indexpath",
		Root:            "root",
		Meta:            afind.Meta{"foo": "bar"},
		State:           afind.OK,
		NumFiles:        123,
		SizeIndex:       afind.ByteSize(131072),
		SizeData:        afind.ByteSize(1048576),
		NumShards:       4,
		ElapsedIndexing: time.Duration(1 * time.Second),
	}
)

func TestIndex(t *testing.T) {
	sys := newRpcServer(t, getTestConfig())
	addr := sys.rpcServer.l.Addr().String()
	defer sys.rpcServer.CloseNoErr()

	cl, err := NewRpcClient(addr)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	afindd := NewIndexerClient(cl)
	defer afindd.Close()

	// Try some bogus queries first to check validation
	query := afind.NewIndexQuery("")
	ir, err := afindd.Index(context.Background(), query)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	experr := "Argument 'key' value is invalid: Value must not be empty"
	if ir.Error.Message() != experr {
		t.Error("want empty key error, got", ir.Error)
	}

	query = afind.NewIndexQuery("key")
	query.Dirs = []string{"."}
	query.Meta = afind.Meta{"foo": "bar"}
	query.Root = "/var/local/src/foo"
	ir, err = afindd.Index(context.Background(), query)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if !reflect.DeepEqual(ir.Repo, ktIndexRepo1) {
		t.Error("did not get expected repo")
	}
	if called := sys.indexer.called["Index"]; called != 1 {
		t.Error("want index mock called once, got", called, "times")
	}
}
