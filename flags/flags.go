package flags

import (
	"fmt"
	"strings"

	"github.com/andaru/afind/errs"
)

// Command line 'flag' types used by both afind (CLI tool) and afindd
// (daemon)

// Slice of strings flag
type StringSlice []string

func (ss *StringSlice) String() string {
	return fmt.Sprint(*ss)
}

func (ss *StringSlice) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		*ss = append(*ss, v)
	}
	return nil
}

func (ss *StringSlice) AsSliceOfString() []string {
	return *ss
}

// String/string map flag
// This is used for user defined repo metadata by afind and afindd
type SSMap map[string]string

// Returns a go default formatted form of the metadata map flag
func (ssmap *SSMap) String() string {
	return fmt.Sprint(*ssmap)
}

func (ssmap *SSMap) Set(value string) error {
	kv := strings.Split(value, "=")
	if len(kv) != 2 {
		return errs.NewValueError("-D", "must be in the form 'key=value'")
	}
	s := *ssmap
	s[kv[0]] = kv[1]
	return nil
}
