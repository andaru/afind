package afind

import (
	"os"
	"testing"
)

func TestDbGetSetDelete(t *testing.T) {
	db := newDb()
	key := "TestDbGetSetDelete"
	if db.Get(key) != nil {
		t.Error("should get nil for not present keys")
	}
	repo := &Repo{Key: "foo"}
	db.Set(key, repo)
	if v := db.Get(key); v == nil {
		t.Error("should have a key now")
	} else if v.(*Repo).Key != "foo" {
		t.Error("got", v.(*Repo).Key, " want 'foo'")
	}
	// Delete the entry using either approach
	db.Delete(key)
	if db.Get(key) != nil {
		t.Error("should get nil for not present keys")
	}
	db.Set(key, repo)
	if db.Get(key) == nil {
		t.Error("should have a key now")
	}
	db.Set(key, nil)
	if db.Get(key) != nil {
		t.Error("should get nil for not present keys")
	}
}

func TestForEach(t *testing.T) {
	d := newDb()
	repos := map[string]*Repo{
		`1_001`: &Repo{Key: `1_001`},
		`1_002`: &Repo{Key: `1_002`},
		`1_003`: &Repo{Key: `1_003`},
	}

	for k, r := range repos {
		d.Set(k, r)
	}

	count := 0
	// test iterator to completion
	d.ForEach(func(key string, value interface{}) bool {
		if v := d.Get(key); v == nil {
			t.Error("could not find value for iterable key", key)
		}
		count++
		return true
	})
	if count != 3 {
		t.Error("want 3 Repo, got", count)
	}

	// test exiting from the iterator early
	count = 0
	d.ForEach(func(key string, value interface{}) bool {
		count++
		return false
	})
	if count != 1 {
		t.Error("want count == 1, not count ==", count)
	}
}

func TestDbSize(t *testing.T) {
	d := newDb()
	repos := map[string]*Repo{
		`1_001`: &Repo{Key: `1_001`},
		`1_002`: &Repo{Key: `1_002`},
		`1_003`: &Repo{Key: `1_003`},
	}

	for k, r := range repos {
		d.Set(k, r)
	}
	if s := d.Size(); s != 3 {
		t.Error("got", s, "repos, want 3")
	}
}

func TestBackedDb(t *testing.T) {
	fn := "./backed.json"
	_ = os.Remove(fn)
	d := newJsonBackedDb(fn)
	defer os.Remove(fn)

	d.Set("1", &Repo{Key: "1"})
	fi, err := os.Stat(fn)
	if err != nil {
		t.Error("unexpected error:", err.Error())
	}
	if fi.Size() < 100 {
		t.Error("size of", fn, "was < 100 bytes, want more")
	}

	// Now cause the database to be re-read from disk and confirm
	// the value still exists within.
	d2 := newJsonBackedDb(fn)
	v := d2.Get("1")
	if value, ok := v.(*Repo); !ok {
		t.Logf("%#v", v)
		t.Error("want a *Repo, got something else")
	} else {
		if value.Key != "1" {
			t.Error("want key=='1', got", value.Key)
		}
	}
	if err := d2.close(); err != nil {
		t.Error("unexpected error on close():", err.Error())
	}
}

func TestDbConstructors(t *testing.T) {
	memdb := NewDb()
	filedb := NewJsonBackedDb("./test_db_constructor.json")
	if memdb.Size() != 0 || filedb.Size() != 0 {
		t.Error("expected empty Size() from file and mem db")
	}
}

func TestDbBadCases(t *testing.T) {
	d := db{}
	if v := d.read(); v != nil {
		t.Error("want nil from d.read(), got", v)
	}
	// "/" is never a file
	d = db{bfn: "/"}
	if v := d.read(); v == nil {
		t.Error("want error from d.read(), got nil")
	}
	if v := d.close(); v == nil {
		t.Error("want error from d.close(), got nil")
	}
	if v := d.flush(); v == nil {
		t.Error("want error from d.flush(), got nil")
	}
}
