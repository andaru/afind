package afind

import (
	"fmt"
	"time"
)

type badStateChangeError struct {
	s string
}

func (e badStateChangeError) Error() string {
	return e.s
}

func BadStateChangeError(s string) badStateChangeError {
	return badStateChangeError{s}
}

type EventTimer interface {
	Start() error
	Stop() error
	Elapsed() time.Duration
}

type Event struct {
	start   time.Time     // Wallclock event start time
	elapsed time.Duration // Event duration
	running bool
}

func (e *Event) Start() error {
	if e.running {
		return BadStateChangeError("Already started")
	}
	e.start = time.Now()
	e.running = true
	return nil
}

func (e *Event) Stop() error {
	if !e.running {
		return BadStateChangeError("Already stopped")
	}
	e.elapsed = time.Since(e.start)
	e.running = false
	return nil
}

func (e *Event) Elapsed() time.Duration {
	return e.elapsed
}

func NewEvent() *Event {
	e := &Event{}
	return e
}

func (e *Event) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%d", e.Elapsed().Nanoseconds())), nil
}
