package afind

import (
	"fmt"
	"strings"
)

// Command line 'flag' types used by both afind (CLI tool) and afindd
// (daemon)

// Slice of strings flag
type FlagStringSlice []string

func (ss *FlagStringSlice) String() string {
	return fmt.Sprint(*ss)
}

func (ss *FlagStringSlice) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		*ss = append(*ss, v)
	}
	return nil
}

func (ss *FlagStringSlice) AsSliceOfString() []string {
	return *ss
}

// String/string map flag
// This is used for user defined repo metadata by afind and afindd
type FlagSSMap map[string]string

// Returns a go default formatted form of the metadata map flag
func (ssmap *FlagSSMap) String() string {
	return fmt.Sprint(*ssmap)
}

func (ssmap *FlagSSMap) Set(value string) error {
	kv := strings.Split(value, "=")
	if len(kv) != 2 {
		return newValueError("-D", "Value must be in the form 'key=value'")
	}
	s := *ssmap
	s[kv[0]] = kv[1]
	return nil
}
