package counters

import (
	"testing"
)

func TestCounters(t *testing.T) {
	counters := New().Start()

	done := make(chan bool)
	n := 20000
	for i := 0; i < n; i++ {
		go func() {
			counters.Inc("key2", 50)
			counters.Inc("key1", 100)
			counters.Inc("key2", 50)
			counters.Inc("key3", 1)
			counters.Inc("", 100)
			counters.Inc("nothin", 0)
			done <- true
		}()
	}
	left := n
	for left > 0 {
		<-done
		left--
	}

	ctrs := counters.GetAll()
	if len(ctrs) != 3 {
		// should have key1..key3
		t.Error("want 3 keys in counter map, got", len(ctrs))
	}

	key1 := counters.Get("key1")
	exp := uint64(n * 100)
	if ctrs["key1"] != exp || key1 != exp {
		t.Error("for key1: got", key1, "want", exp)
	}

	key2 := counters.Get("key2")
	if ctrs["key2"] != exp || key2 != exp {
		t.Error("for key2: got", key2, "want", exp)
	}

	key3 := counters.Get("key3")
	exp = uint64(n)
	if ctrs["key3"] != exp || key3 != exp {
		t.Error("for key3: got", key3, "want", n*1)
	}

	notexist := counters.Get("notexist")
	if notexist != 0 {
		t.Error("for notexist: got", notexist, "want 0")
	}

	emptykey := counters.Get("")
	if emptykey != 0 {
		t.Error("for key '': got", emptykey, "want 0")
	}

	if err := counters.Close(); err != nil {
		t.Error("expected no error from Close(), got", err)
	}
}
