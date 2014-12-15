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

func TestGetPreCtx(t *testing.T) {
	var b []byte
	b = []byte(string("foo\nbar\nbaz"))
	v := getPreCtx(b, 2, 0, 4)
	if len(v) != 1 {
		t.Error("want 1 line of pre context, got", len(v))
	}
	eq(t, "foo\n", string(v[0]))

	b = []byte(string("foo\nbar\nbaz"))
	v = getPreCtx(b, 2, 0, 8)
	if len(v) != 2 {
		t.Error("want 2 line of pre context, got", len(v))
	}
	eq(t, "bar\n", string(v[0]))
	eq(t, "foo\n", string(v[1]))

	b = []byte(string("foo\nbar\nbaz\nqux"))
	v = getPreCtx(b, 2, 0, 12)
	if len(v) != 2 {
		t.Error("want 2 line of pre context, got", len(v))
	}
	eq(t, "baz\n", string(v[0]))
	eq(t, "bar\n", string(v[1]))

	b = []byte(string(""))
	v = getPreCtx(b, 2, 0, 12)
	if len(v) != 0 {
		t.Error("want 0 line of pre context, got", len(v))
	}

	b = []byte(string("foo\nbar\nbaz\nqux"))
	v = getPreCtx(b, 0, 0, 0)
	if len(v) != 0 {
		t.Error("want 0 line of pre context, got", len(v))
	}

	b = []byte(string(""))
	v = getPreCtx(b, 0, 0, 0)
	if len(v) != 0 {
		t.Error("want 0 line of pre context, got", len(v))
	}

}

func TestGetPostCtx(t *testing.T) {
	var b []byte
	b = []byte(string("foo\nbar\nbaz"))
	v := getPostCtx(b, 0, 4, len(b))
	if len(v) != 0 {
		t.Error("want 0 line of post context, got", len(v))
	}

	b = []byte(string("foo\nbar\nbaz"))
	v = getPostCtx(b, 1, 4, len(b))
	if len(v) != 1 {
		t.Error("want 1 line of post context, got", len(v))
	}
	eq(t, "baz", string(v[0]))

	b = []byte(string("foo\nbar\nbaz\nqux\n"))
	v = getPostCtx(b, 1, 4, len(b))
	if len(v) != 1 {
		t.Error("want 1 line of post context, got", len(v))
	}
	eq(t, "baz\n", string(v[0]))

	b = []byte(string("foo\nbar\nbaz\nqux\n"))
	v = getPostCtx(b, 2, 4, len(b))
	if len(v) != 2 {
		t.Error("want 2 lines of post context, got", len(v))
	}
	eq(t, "baz\n", string(v[0]))
	eq(t, "qux\n", string(v[1]))

	b = []byte(string("foo\nbar\nbaz\nqux\n"))
	v = getPostCtx(b, 1000, 0, len(b))
	if len(v) != 3 {
		t.Error("want 3 lines of post context, got", len(v))
	}
	eq(t, "bar\n", string(v[0]))
	eq(t, "baz\n", string(v[1]))
	eq(t, "qux\n", string(v[2]))
}
