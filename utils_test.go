package afind

import (
	"path/filepath"
	"testing"
)

func TestNormalizeUrlLocal(t *testing.T) {
	uri := "/tmp/foo"
	normUri, err := normalizeUri(uri)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if normUri != "/tmp/foo" {
		t.Error("got ", normUri, " want ", uri)
	}
}

func TestNormalizeUrlLocalIsMadeAbsolute(t *testing.T) {
	uri := "./foo"
	normUri, err := normalizeUri(uri)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	absUri, err := filepath.Abs(uri)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if normUri != absUri {
		t.Error("got ", normUri, " want ", absUri)
	}

}
