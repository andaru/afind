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
	ir := NewIndexQuery("key")
	//ir := NewIndexQuery("key", "root", []string{"dir1", "dir2"})
	ir.Dirs = []string{"dir1", "dir2"}
	ir.Root = "/"
	ir.Meta["key"] = "value"
	// should have the same sort of effect as
	ir.Meta["host"] = "xyz"
	// should change the value above, referred to by
	// Repo.Host/Repo.Meta.Host()
	ir.Meta.SetHost("abc")
	r2 := newRepoFromQuery(&ir, "/")

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

func TestRepoSetMeta(t *testing.T) {
	defaults := Meta{"host": "defaulthost"}
	r1 := NewRepo()
	r1.Meta.SetHost("abc") // will be replaced by defaulthost
	r1.Meta["foo"] = "bar"

	// host below is replacing abc
	reqMeta := Meta{"project": "foo", "host": "final"}

	r1.SetMeta(defaults, reqMeta)
	if len(r1.Meta) != 3 {
		t.Error("want 3 keys in Meta, got", r1.Meta)
	}
	eq(t, "bar", r1.Meta["foo"])
	eq(t, "final", r1.Meta.Host())
	eq(t, "final", r1.Host())
	eq(t, "foo", r1.Meta["project"])
}

func TestRepoJson(t *testing.T) {
	r := Repo{Meta: make(map[string]string)}
	r.Root = "root"
	r.Meta["foo"] = " bar "
	r.Key = "key"
	r.IndexPath = "indexpath"
	r.SetHost("m123.foo")
	r.State = INDEXING
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

	// workaround: private field `loc` in struct time.Time has a
	// pointer to a location, rather than nil, which are both
	// represented as 0 unix time in UTC. Copy its version so
	// DeepEqual works.
	newr.TimeUpdated = r.TimeUpdated
	if !reflect.DeepEqual(r, newr) {
		t.Logf("self=%#v", r)
		t.Logf("other=%#v", newr)
		t.Logf("created %v %v", r.TimeUpdated, newr.TimeUpdated)
		t.Error("Repo lost data during marshal/unmarshal")
	}

	// Some additional paranoid checks
	eq(t, " bar ", newr.Meta["foo"])
	eq(t, "m123.foo", newr.Host())
	eq(t, "INDEXING", newr.State)
	eq(t, "key", newr.Key)
	eq(t, 33, newr.NumFiles)
	eq(t, 6, newr.NumShards)
}
