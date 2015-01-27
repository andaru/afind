package walkablefs

import (
	"os"
	"testing"

	"golang.org/x/tools/godoc/vfs/mapfs"
)

func TestWalker(t *testing.T) {
	count := 0
	var lastErr error
	var lastP string
	walked := make(map[string]struct{})

	files := map[string]string{
		"bar/baz/foo.go": "package baz",
		"Makefile.am":    "top level Makefile",
	}

	wfunc := func(p string, info os.FileInfo, err error) error {
		walked[p] = struct{}{}
		lastErr = err
		lastP = p
		count++
		return nil
	}

	// Build a walkable filesystem from the map data in 'files'
	wfs := New(mapfs.New(files))

	// Confirm we get an error on a non-existant path,
	// and that we are notified of the error by having our
	// callback called once with the error set for the root
	// path we passed in.
	_ = wfs.Walk("bar", wfunc)
	if count != 1 {
		t.Error("want 1 path, got", count)
	} else if lastErr == nil || lastP != "bar" {
		t.Error("want an error for path 'bar'")
	}

	// Now put some files in the system and walk it again.

	// From the two entries we've added to 'files' above,
	// we'll get a root, a bar/, a bar/baz/, a bar/baz/foo.go,
	// and a Makefile.am, all prefixed with a root slash.
	count = 0
	_ = wfs.Walk("/", wfunc)
	if count != 5 {
		t.Error("want 5 paths, got", count)
	}
	for _, name := range []string{
		"/",
		"/bar",
		"/bar/baz",
		"/bar/baz/foo.go",
		"/Makefile.am",
	} {
		if _, ok := walked[name]; !ok {
			t.Error("expected", name, "in files walked, but was not")
		}
	}

}
