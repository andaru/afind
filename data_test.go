package afind

import (
	"bytes"
	"fmt"
	"testing"
)

func TestByteSize(t *testing.T) {
	check := func(nb int, exp string) {
		bs := ByteSize(nb)
		if bs.String() != exp {
			t.Errorf("%v bytes: got %v, want %v", nb, bs.String(), exp)
		}
		v, err := bs.MarshalJSON()
		if err != nil {
			t.Error("unexpected error:", err.Error())
		}
		bexp := []byte(fmt.Sprintf("%.f", bs))
		if !bytes.Equal(v, bexp) {
			t.Errorf("MarshalJSON: got %v (%v), want %v (%v)",
				v, string(v), bexp, string(bexp))
		}

		vt, err := bs.MarshalText()
		if err != nil {
			t.Error("unexpected error:", err.Error())
		}
		bexpt := []byte(bs.String())
		if !bytes.Equal(vt, bexpt) {
			t.Errorf("MarshalText: got %v (%v), want %v (%v)",
				vt, string(vt), bexpt, string(bexpt))
		}
	}
	check(1, "1B")
	check(1000, "1000B")
	check(1023, "1023B")
	check(1024, "1.00KB")
	check(1024*1024, "1.00MB")
	check(1024*1024*1024, "1.00GB")
	check(1024*1024*1024*1024, "1.00TB")
}
