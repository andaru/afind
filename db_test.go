package afind

import "testing"

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
