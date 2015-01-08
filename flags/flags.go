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
// When the map is empty, returns the helper string "key=value",
// to give a hint for users as to the valid format. e.g.,
//
//   -m=key=value: A key value pair found in Repo to search
func (ssmap *SSMap) String() string {
	if len(*ssmap) == 0 {
		return "key=value"
	}
	return fmt.Sprint(*ssmap)
}

func (ssmap *SSMap) Set(value string) error {
	kv := strings.Split(value, "=")
	if len(kv) != 2 {
		return errs.NewValueError("meta", "Value must ust be in the form \"k=v\"")
	}
	s := *ssmap
	s[kv[0]] = kv[1]
	return nil
}
