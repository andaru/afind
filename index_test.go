package afind

import "testing"

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

func TestMakeIndex(t *testing.T) {
	req := IndexRequest{Meta: map[string]string{"project": "Foo"}}
	var resp *IndexResponse
	var err error

	resp, err = makeIndex(req)

	if err == nil {
		t.Error("Expected error; got none")
	}

	req.Key = "some-key"
	req.Root = "./testdata/repo1"
	req.Dirs = []string{"."}
	resp, err = makeIndex(req)
	if err != nil {
		t.Error("Expected no error; got", err)
	}
	if len(resp.Repos) != 1 {
		t.Error("Want 1 repo, got %d repos", len(resp.Repos))
	}
	for _, resprepo := range resp.Repos {
		if v, ok := resprepo.Meta["project"]; !ok || v != "Foo" {
			t.Error("Didn't get meta back")
		}
		break
	}
	// todo: check stats
	// t.Logf("resp: %#v", resp)
}
