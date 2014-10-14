package afind

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func cfgIndexTest() Config {
	config := Config{}
	config.IndexInRepo = false
	config.IndexRoot = `/tmp/afind.index.test`
	config.NumShards = 2
	if err := makeIndexRoot(config); err != nil {
		log.Fatalf("Could not make IndexRoot: %v", err)
	}
	return config
}

func endIndexTest(config *Config) {
	if err := os.RemoveAll(config.IndexRoot); err != nil {
		log.Critical(err.Error())
	}
}

func TestNewIndexRequestWithMeta(t *testing.T) {
	cfg := cfgIndexTest()
	defer endIndexTest(&cfg)

	ir := newIndexRequest("key", "root", []string{"foo", "foo/bar"})
	if ir.Key != "key" {
		t.Errorf("got %v want key", ir.Key)
	}
	if ir.Root != "root" {
		t.Errorf("got %v want root", ir.Root)
	}
	if ir.Dirs[0] != "foo" {
		t.Errorf("got %v want foo", ir.Dirs[0])
	}
	if ir.Dirs[1] != "foo/bar" {
		t.Errorf("got %v want foo/bar", ir.Dirs[1])
	}
	if len(ir.Meta) != 0 {
		t.Errorf("got %v want empty Meta", ir.Meta)
	}
}

func getTempDirWithFile(testname string) (dir string, f *os.File, err error) {
	dir, err = ioutil.TempDir("/tmp", "test.afind")
	if err != nil {
		return
	}
	f, err = ioutil.TempFile(dir, testname)
	return
}

func getTempDir(testname string) (dir string, err error) {
	return ioutil.TempDir("/tmp", "test.afind")
}

func TestIndex(t *testing.T) {
	cfg := cfgIndexTest()
	defer endIndexTest(&cfg)

	svc := NewService(newDb(), cfg)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	req := IndexRequest{Meta: map[string]string{
		"hostname": "afind123",
		"project":  "Foo",
	}}
	var resp *IndexResponse

	req.Key = "1234"
	req.Root = path.Join(cwd, "./testdata/repo1/")
	req.Dirs = []string{"."}

	ixr := newIndexer(*svc)
	resp, err = ixr.Index(req)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if resp.Repo.SizeData == 0 {
		t.Error("want >0 bytes data")
	}
	if resp.Repo.SizeIndex < 1 {
		t.Error("want >0 bytes index")
	}
	if v, ok := resp.Repo.Meta["project"]; !ok || v != "Foo" {
		t.Error("Didn't get meta back")
	}
}
