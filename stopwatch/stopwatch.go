package stopwatch

import (
	"time"
)

// StopWatcher is an interface to a collection of named stop
// watches.
type StopWatcher interface {
	// Start creates a new watch, returning the creation time.
	// This panics if an existing duplicate named watch has been
	// started and not yet stopped.
	Start(name string) *time.Time

	// Stop stops and deletes the watch, returning the time since
	// Start was called. This panics if no such named watch has
	// yet been started.
	Stop(name string) *time.Duration
}

type stopwatches map[string]time.Time

func (sw stopwatches) Start(name string) (x time.Time) {
	if _, ok := sw[name]; !ok {
		x = time.Now()
		sw[name] = x
		return
	}
	panic("stopwatch can only be started once:" + name)
}

func (sw stopwatches) Stop(name string) time.Duration {
	if started, ok := sw[name]; ok {
		delete(sw, name)
		return time.Since(started)
	}
	panic("stopwatch not started:" + name)
}

func New() stopwatches {
	return stopwatches{}
}
