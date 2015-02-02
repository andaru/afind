package afind

import (
	"io"
	"sync"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/walkablefs"
	"github.com/andaru/codesearch/index"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

type mockIndexWriter struct {
	name   string
	called map[string]int
	lock   *sync.Mutex
}

type mockIndices map[string]*mockIndexWriter

var (
	mockIx mockIndices = mockIndices{}
)

func (ix mockIndices) calls(key string) int {
	sum := 0
	for _, mock := range ix {
		sum += mock.called[key]
	}
	return sum
}

func (ix *mockIndices) reset() {
	for _, mock := range *ix {
		mock.called = make(map[string]int)
	}
}

func testSearchContext(fs walkablefs.WalkableFileSystem) context.Context {
	return context.WithValue(context.Background(), "FileSystem", fs)
}

func testIndexContext(fs walkablefs.WalkableFileSystem) context.Context {
	ctx := context.WithValue(context.Background(), "FileSystem", fs)
	return context.WithValue(ctx, "IndexWriterFunc", getWriterFunc())
}

type mockFileSystem struct {
	walkablefs.WalkableFileSystem
	lock *sync.Mutex
}

func getMockFs(files map[string]string) *mockFileSystem {
	return &mockFileSystem{
		walkablefs.New(mapfs.New(files)),
		&sync.Mutex{}}
}

func getWriterFunc() func(string) index.IndexWriter {
	return func(name string) index.IndexWriter {
		log.Debug("writerFunc for %v", name)
		mock := &mockIndexWriter{name, map[string]int{}, &sync.Mutex{}}
		mockIx[name] = mock
		return mock
	}
}

const (
	cDataBytes  = 1000
	cIndexBytes = 123
)

func (iw mockIndexWriter) reset() {
	iw.lock.Lock()
	defer iw.lock.Unlock()
	iw.called = make(map[string]int)
}

func (iw mockIndexWriter) AddPaths(paths []string) {
	iw.lock.Lock()
	defer iw.lock.Unlock()
	iw.called["AddPaths"]++
}

func (iw mockIndexWriter) AddFile(name string) {
	iw.lock.Lock()
	defer iw.lock.Unlock()
	iw.called["AddFile"]++
}

func (iw mockIndexWriter) Add(name string, f io.Reader) {
	iw.lock.Lock()
	defer iw.lock.Unlock()
	iw.called["Add"]++
}

func (iw mockIndexWriter) DataBytes() int64 {
	iw.lock.Lock()
	defer iw.lock.Unlock()
	// DataBytes and IndexBytes can be called in log calls,
	// affecting call counters.
	iw.called["DataBytes"]++
	return cDataBytes
}

func (iw mockIndexWriter) IndexBytes() uint32 {
	iw.lock.Lock()
	defer iw.lock.Unlock()
	iw.called["IndexBytes"]++
	return cIndexBytes
}

func (iw mockIndexWriter) Flush() {
	iw.lock.Lock()
	defer iw.lock.Unlock()
	iw.called["Flush"]++
}
