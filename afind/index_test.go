package afind

import (
	"testing"
)

func TestGetRoot(t *testing.T) {
	c := &Config{IndexInRepo: true}
	q := NewIndexQuery("key")

	// switch back and forth between trailing and non-trailing
	// slash terminated Root paths, and confirm we see the same
	// result.
	exp1 := "/var/path"
	q.Root = "/var/path/"
	eq(t, exp1, getRoot(c, &q))
	q.Root = "/var/path"
	eq(t, exp1, getRoot(c, &q))
	q.Root = "/var/path/"
	eq(t, exp1, getRoot(c, &q))

	// try the same thing, but when not indexing in the repo,
	// which will cause a different path calculation.
	c.IndexInRepo = false
	c.IndexRoot = "/tmp/root/"
	exp2 := "/tmp/root/key"
	eq(t, exp2, getRoot(c, &q))
	c.IndexRoot = "/tmp/root"
	eq(t, exp2, getRoot(c, &q))
	c.IndexRoot = "/tmp/root/"
}

func TestRootStripper(t *testing.T) {
	check := func(exp, actual string) {
		if exp != actual {
			t.Error("got", actual, "want", exp)
		}
	}

	rs := newRootStripper("/trailingslash/")
	check("foobar.txt", rs.suffix("/trailingslash/foobar.txt"))

	rs = newRootStripper("/noslash")
	check("foobar.txt", rs.suffix("/noslash/foobar.txt"))
}
