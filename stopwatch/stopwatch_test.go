package stopwatch

import (
	"testing"
	"time"
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
	done := make(chan struct{}, 1)
	defer func() {
		if r := recover(); r != nil {
			done <- struct{}{}
		}
	}()
	New().Stop("will-panic")
	timeout := time.After(time.Duration(10 * time.Millisecond))
	select {
	case <-timeout:
		t.Error("did not panic as expected")
	default:
	}
}
