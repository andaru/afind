package afind

import (
	"testing"
)

func TestCountNL(t *testing.T) {
	var b []byte
	eq(t, 0, countNL(b))
	b = []byte(string("foo"))
	eq(t, 0, countNL(b))
	b = []byte(string("foo\n"))
	eq(t, 1, countNL(b))
	b = []byte(string("foo\nbar"))
	eq(t, 1, countNL(b))
	b = []byte(string("foo\nbar\n"))
	eq(t, 2, countNL(b))
	b = []byte(string("foo\nbar\nbaz\n"))
	eq(t, 3, countNL(b))
}
