package afind

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMeta(t *testing.T) {
	m1 := Meta{"foo": "bar"}
	m2 := Meta{}
	other := Meta{"newfoo": "baz"}
	m1.Update(other)
	m2.Update(other)
	expected1 := Meta{"foo": "bar", "newfoo": "baz"}

	if !m1.Matches(expected1) {
		t.Errorf("got %v, want %v", m1, expected1)
	}
	if !m2.Matches(other) {
		t.Errorf("got %v, want %v", m1, other)
	}
}

func TestMetaPointer(t *testing.T) {
	m1 := Meta{"foo": "bar"}
	m2 := &Meta{}
	other := Meta{"newfoo": "baz"}
	m1.Update(other)
	m2.Update(other)
	expected1 := &Meta{"foo": "bar", "newfoo": "baz"}

	if !m1.Matches(*expected1) {
		t.Errorf("got %v, want %v", m1, expected1)
	}
	if !m2.Matches(other) {
		t.Errorf("got %v, want %v", m1, other)
	}
}

func TestRepoSetup(t *testing.T) {
	r1 := NewRepo()
	ir := NewIndexQuery("key", "root", []string{"dir1", "dir2"})
	ir.Meta["key"] = "value"
	ir.Meta["host"] = "abc"
	r2 := NewRepoFromQuery(&ir, "/")

	if r1.Host() != "" || r2.Host() != "abc" {
		t.Error("Host() had unexpected results; r1=",
			r1.Host(), " r2=", r2.Host())
	}
	r2.NumShards = 2
	shards := r2.Shards()
	if len(shards) != 2 {
		t.Error("got", len(shards), "shards, want 2")
	}
}

func TestRepoJson(t *testing.T) {
	r := Repo{Meta: make(map[string]string)}
	r.Root = "root"
	r.Meta["foo"] = " bar "
	r.Key = "key"
	r.IndexPath = "indexpath"
	r.SetHost("m123.foo")
	r.State = INDEXING
	r.NumDirs = 22
	r.NumFiles = 33
	r.NumShards = 6

	b, err := json.Marshal(r)
	if err != nil {
		t.Errorf("unexpected json marshal error: %v", err)
	}

	newr := Repo{}
	err = json.Unmarshal(b, &newr)
	if err != nil {
		t.Errorf("unexpected json unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(r, newr) {
		t.Error("Repo lost data during marshal/unmarshal")
	}

	// Some additional paranoid checks
	eq(t, " bar ", newr.Meta["foo"])
	eq(t, "m123.foo", newr.Host())
	eq(t, "INDEXING", newr.State)
	eq(t, "key", newr.Key)
	eq(t, 22, newr.NumDirs)
	eq(t, 33, newr.NumFiles)
	eq(t, 6, newr.NumShards)
}
