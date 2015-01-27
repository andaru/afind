package afind

import (
	"io"
	"strings"
	"testing"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/walkablefs"
	"github.com/andaru/codesearch/index"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

type mockIndexWriter struct {
	name   string
	t      *testing.T
	called map[string]int
}

func getMock(name string, t *testing.T) *mockIndexWriter {
	return &mockIndexWriter{name, t, map[string]int{}}
}

const (
	cDataBytes  = 1000
	cIndexBytes = 123
)

func (iw mockIndexWriter) reset() {
	iw.called = make(map[string]int)
}

func (iw mockIndexWriter) AddPaths(paths []string) {
	iw.called["AddPaths"]++
}

func (iw mockIndexWriter) AddFile(name string) {
	iw.called["AddFile"]++
}

func (iw mockIndexWriter) Add(name string, f io.Reader) {
	iw.called["Add"]++
}

func (iw mockIndexWriter) DataBytes() int64 {
	// DataBytes and IndexBytes can be called in log calls,
	// affecting call counters.
	iw.called["DataBytes"]++
	return cDataBytes
}

func (iw mockIndexWriter) IndexBytes() uint32 {
	iw.called["IndexBytes"]++
	return cIndexBytes
}

func (iw mockIndexWriter) Flush() {
	iw.called["Flush"]++
}

func TestGetRoot(t *testing.T) {
	c := &Config{IndexInRepo: true}
	q := NewIndexQuery("key")

	// switch back and forth between trailing and non-trailing
	// slash terminated Root paths, and confirm we see the same
	// result.
	exp1 := "/var/path"
	q.Root = "/var/path/"
	eq(t, exp1, getRoot(c, &q))
	q.Root = "/var/path"
	eq(t, exp1, getRoot(c, &q))
	q.Root = "/var/path/"
	eq(t, exp1, getRoot(c, &q))

	// try the same thing, but when not indexing in the repo,
	// which will cause a different path calculation.
	c.IndexInRepo = false
	c.IndexRoot = "/tmp/root/"
	exp2 := "/tmp/root/key"
	eq(t, exp2, getRoot(c, &q))
	c.IndexRoot = "/tmp/root"
	eq(t, exp2, getRoot(c, &q))
	c.IndexRoot = "/tmp/root/"
}

type ourError struct{}

func (ourError) Error() string {
	return "our error"
}

func TestSetError(t *testing.T) {
	check := func(res *IndexResult) {
		if res.Error == nil {
			t.Error("got nil, want non-nil")
		}
	}

	e := errs.NewStructError(errs.NewRepoUnavailableError())
	res := NewIndexResult()
	res.SetError(e)
	check(res)

	res = NewIndexResult()
	res.SetError(ourError{})
	check(res)
}

func TestNormalize(t *testing.T) {
	q := NewIndexQuery("key")
	// Pull the key off, first
	q.Key = ""
	if err := q.Normalize(); err != nil && !errs.IsValueError(err) {
		t.Errorf("want a ValueError, got %v", err)
	} else if err != nil && !strings.Contains(err.Error(), "Key must not be empty") {
		t.Errorf("want a ValueError about the key being empty, got %v", err)
	}

	// Fix the first error
	q.Key = "key"
	if err := q.Normalize(); err != nil && !errs.IsValueError(err) {
		t.Errorf("want a ValueError, got %v", err)
	}
	// Pass the second check, about Dirs
	q.Dirs = []string{"."}

	// But fail the third test...
	q.Root = ".not_absolute"
	if err := q.Normalize(); err != nil && !errs.IsValueError(err) {
		t.Errorf("want a ValueError, got %v", err)
	} else if err != nil && !strings.Contains(err.Error(), "Root must be an absolute path") {
		t.Errorf("want a ValueError about an absolute path, got %v", err)
	}

	// Now pass the second and fail the third test, that the Dirs
	// must be relative paths.
	q.Root = "/"
	q.Dirs = []string{".", "/"}
	if err := q.Normalize(); err != nil && !errs.IsValueError(err) {
		t.Errorf("want a ValueError, got %v", err)
	} else if err != nil && !strings.Contains(err.Error(), "Dirs must not be absolute paths") {
		t.Errorf("want a ValueError about an Dirs being absolute, got %v", err)
	}

}

func TestIndexerBasic(t *testing.T) {
	files := map[string]string{
		"src/foo/foo.go": "package foo\n",
		"README":         "Root directory README file\n\n",
	}
	c := &Config{IndexInRepo: true, NumShards: 1}
	db := newDb()
	wfs := walkablefs.New(mapfs.New(files))
	ix := NewIndexer(c, db)

	var mock *mockIndexWriter
	writerFunc := func(name string) index.IndexWriter {
		mock = getMock(name, t)
		mock.reset()
		return mock
	}

	ctx := context.WithValue(context.Background(), "FileSystem", wfs)
	ctx = context.WithValue(ctx, "IndexWriterFunc", writerFunc)

	query := NewIndexQuery("key2")
	query.Dirs = []string{"."}
	query.Root = "/"
	_, err := ix.Index(ctx, query)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	// check that the appropriate functions were mocked
	for _, f := range []string{"Flush", "DataBytes", "IndexBytes"} {
		if mock.called[f] != 1 {
			t.Error("want", f, "called 1 time, got", mock.called[f])
		}
	}

	if mock.called["Add"] != 2 {
		t.Error("want Add called 2 times, got", mock.called["Add"])
	}
}

func TestIndexerSharding(t *testing.T) {
	files := map[string]string{
		"src/foo/foo.go": "package foo\n",
		"README":         "Root directory README file\n\n",
	}
	// Confirm we get at least one shard when asking for 0 (the
	// default NumShards)
	c := &Config{IndexInRepo: true}
	db := newDb()
	wfs := walkablefs.New(mapfs.New(files))
	ix := NewIndexer(c, db)

	var mock *mockIndexWriter
	writerFunc := func(name string) index.IndexWriter {
		mock = getMock(name, t)
		return mock
	}

	ctx := context.WithValue(context.Background(), "FileSystem", wfs)
	ctx = context.WithValue(ctx, "IndexWriterFunc", writerFunc)

	query := NewIndexQuery("key1")
	query.Dirs = []string{"."}
	query.Root = "/"
	resp, err := ix.Index(ctx, query)

	nshards := resp.Repo.NumShards
	if err != nil {
		t.Error("unexpected error:", err)
	} else if resp.Repo.SizeData != ByteSize(nshards*cDataBytes) {
		t.Error("got", resp.Repo.SizeData, "bytes data, want", ByteSize(nshards*cDataBytes))
	}

	// Set 2 shards and confirm the index size grows
	ix.cfg.NumShards = 2
	resp, err = ix.Index(ctx, query)
	nshards = resp.Repo.NumShards
	if err != nil {
		t.Error("unexpected error:", err)
	} else if resp.Repo.SizeData != ByteSize(nshards*cDataBytes) {
		t.Error("got", resp.Repo.SizeData, "bytes data, want", ByteSize(nshards*cDataBytes))
	} else if resp.Repo.NumFiles != 2 {
		t.Error("want 2 files, got", resp.Repo.NumFiles)
	}
}
