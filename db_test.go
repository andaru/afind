package afind

import (
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
