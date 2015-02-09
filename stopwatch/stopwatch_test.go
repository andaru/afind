package stopwatch

import (
	"testing"
)

func TestStopWatch(t *testing.T) {
	sw := New()
	// Normal operation
	start1 := sw.Start("test1")
	duration1 := sw.Stop("test1")
	if start1.IsZero() {
		t.Error("expected but did not get a non-zero start1 time")
	}
	if duration1 == 0 {
		t.Error("expected but did not get a non-zero duration")
	}
}

func TestStopWatchPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			exp := "not started: will-panic"
			if act := r.(string); act != exp {
				t.Errorf("want %s, got %s", exp, act)
			}
		} else {
			t.Error("wanted: a panic, got none")
		}
	}()
	sw := New()
	sw.Stop("will-panic")
}

func TestStopWatchDoubleStart(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			exp := "already started: test"
			if act := r.(string); act != exp {
				t.Errorf("want %s, got %s", exp, act)
			}
		} else {
			t.Error("wanted: a panic, got none")
		}
	}()
	sw := New()
	tt := sw.Start("test")
	if tt.IsZero() {
		t.Error("got zero stopwatch start time, want non-zero")
	}
	// This will trip the panic for a stopwatch being started twice
	sw.Start("test")
}
