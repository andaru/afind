package afind

import (
	"io/ioutil"
	"os"
	"testing"
)

func assertEqual(t *testing.T, a, b interface{}) {
	if a != b {
		t.Errorf("%v != %v", a, b)
	}
}

func TestNewIndexRequest(t *testing.T) {
}

func TestNewIndexRequestWithMeta(t *testing.T) {
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

func TestMakeIndex(t *testing.T) {
	req := IndexRequest{Meta: map[string]string{"project": "Foo"}}
	var resp *IndexResponse
	var dir string
	var err error
	var file *os.File

	dir, file, err = getTempDirWithFile("TestMakeIndex")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	defer os.RemoveAll(dir)

	req.Key = "1234"
	req.Root = dir
	req.Dirs = []string{"."}
	resp, err = makeIndex(req, file)

	if len(resp.Repos) != 1 {
		t.Errorf("Want 1 repo, got %d repos", len(resp.Repos))
	}
	for _, resprepo := range resp.Repos {
		// we indexed no files
		if resprepo.SizeData != 0 {
			t.Error("want 0 bytes data, got", resprepo.SizeData)
		}
		if resprepo.SizeIndex < 1 {
			t.Error("want >0 bytes index")
		}
		if v, ok := resprepo.Meta["project"]; !ok || v != "Foo" {
			t.Error("Didn't get meta back")
		}
		break
	}
}
