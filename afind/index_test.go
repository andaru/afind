package afind

import (
	"testing"

	"github.com/andaru/afind/errs"
	"strings"
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

type ourError struct{}

func (ourError) Error() string {
	return "our error"
}

func TestSetError(t *testing.T) {
	check := func(res *IndexResult) {
		if res.Error == nil {
			t.Error("got nil, want non-nil")
		}
	}

	e := errs.NewStructError(errs.NewRepoUnavailableError())
	res := NewIndexResult()
	res.SetError(e)
	check(res)

	res = NewIndexResult()
	res.SetError(ourError{})
	check(res)
}

func TestNormalize(t *testing.T) {
	q := NewIndexQuery("key")
	// Pull the key off, first
	q.Key = ""
	if err := q.Normalize(); err != nil && !errs.IsValueError(err) {
		t.Errorf("want a ValueError, got %v", err)
	} else if err != nil && !strings.Contains(err.Error(), "Key must not be empty") {
		t.Errorf("want a ValueError about the key being empty, got %v", err)
	}

	// Fix the first error
	q.Key = "key"
	if err := q.Normalize(); err != nil && !errs.IsValueError(err) {
		t.Errorf("want a ValueError, got %v", err)
	}
	// Pass the second check, about Dirs
	q.Dirs = []string{"."}

	// But fail the third test...
	q.Root = ".not_absolute"
	if err := q.Normalize(); err != nil && !errs.IsValueError(err) {
		t.Errorf("want a ValueError, got %v", err)
	} else if err != nil && !strings.Contains(err.Error(), "Root must be an absolute path") {
		t.Errorf("want a ValueError about an absolute path, got %v", err)
	}

	// Now pass the second and fail the third test, that the Dirs
	// must be relative paths.
	q.Root = "/"
	q.Dirs = []string{".", "/"}
	if err := q.Normalize(); err != nil && !errs.IsValueError(err) {
		t.Errorf("want a ValueError, got %v", err)
	} else if err != nil && !strings.Contains(err.Error(), "Dirs must not be absolute paths") {
		t.Errorf("want a ValueError about an Dirs being absolute, got %v", err)
	}

}
