package api

import (
	"testing"

	"github.com/andaru/afind/afind"
)

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
