package afind

import (
	"fmt"
)

type ByteSize float64

const (
	_           = iota // ignore first value by assigning to blank identifier
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
)

func (b ByteSize) String() string {
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}
	return fmt.Sprintf("%.fB", b)
}

func (b ByteSize) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// For JSON, we produce the number of bytes
func (b ByteSize) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.f", b)), nil
}
