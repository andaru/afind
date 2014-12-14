package api

import (
	"testing"

	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/utils"
)

func newConfig() afind.Config {
	return afind.Config{RepoMeta: make(map[string]string)}
}

func eq(t *testing.T, exp, val interface{}) bool {
	if exp != val {
		t.Errorf("want %v, got %v", exp, val)
		return false
	}
	return true
}

func neq(t *testing.T, exp, val interface{}) bool {
	if exp == val {
		t.Errorf("don't want both equal to %v", exp)
		return false
	}
	return true
}

func TestIsLocal(t *testing.T) {
	check := func(config *afind.Config, exp string) bool {
		return isLocal(config, exp)
	}
	assertTrue := func(config *afind.Config, exp string) {
		if !check(config, exp) {
			t.Errorf("expected isLocal true for %v, got false", exp)
		}
	}
	assertFalse := func(config *afind.Config, exp string) {
		if check(config, exp) {
			t.Errorf("expected isLocal true for %v, got false", exp)
		}
	}
	config := &afind.Config{RepoMeta: make(map[string]string)}
	config.RepoMeta["host"] = "testhost"
	assertFalse(config, "notlocalhost")
	assertTrue(config, "")
	assertTrue(config, "127.0.0.1")
	assertTrue(config, "::1")
	assertTrue(config, "localhost")
	assertFalse(config, "bs123")
	assertTrue(config, "testhost")
}

func TestGetAddress(t *testing.T) {
	meta := make(afind.Meta)
	meta["foo"] = "bar"
	meta["host"] = "localhost"

	eq(t, getAddress(meta, ""), "localhost:"+utils.DefaultRpcPort)
	meta["host"] = "foobar"
	eq(t, getAddress(meta, "1234"), "foobar:1234")
}

func TestNewServer(t *testing.T) {
	db := afind.NewDb()
	c := newConfig()
	indexer := afind.NewIndexer(&c, db)
	searcher := afind.NewSearcher(&c, db)
	server := NewServer(db, indexer, searcher, &c)
	neq(t, nil, server)
	eq(t, indexer, server.indexer)
	eq(t, searcher, server.searcher)
}
