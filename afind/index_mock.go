package afind

import (
	"code.google.com/p/go.net/context"
	"time"
)

type testIndexer struct {
	calledIndex bool
	indexKey    string
}

func (i *testIndexer) Index(ctx context.Context, req IndexQuery, ch chan *IndexResult) error {
	i.calledIndex = true
	i.indexKey = req.Key
	ir := NewIndexResult()
	ir.Repo = &Repo{
		Key:             "key",
		IndexPath:       "indexpath",
		Root:            "root",
		Meta:            Meta{"foo": "bar"},
		State:           OK,
		NumFiles:        123,
		SizeIndex:       ByteSize(131072),
		SizeData:        ByteSize(1025 * 1024),
		NumShards:       4,
		ElapsedIndexing: time.Duration(1 * time.Second),
	}
	ch <- ir
	return nil
}
