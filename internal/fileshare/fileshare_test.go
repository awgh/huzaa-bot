package fileshare

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePath(t *testing.T) {
	root, err := os.MkdirTemp("", "fileshare_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	t.Run("valid relative path", func(t *testing.T) {
		p, err := SafePath(root, "foo.txt")
		if err != nil {
			t.Fatal(err)
		}
		if p != filepath.Join(root, "foo.txt") {
			t.Errorf("got %s", p)
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		_, err := SafePath(root, "..")
		if err == nil {
			t.Error("expected error for ..")
		}
		_, err = SafePath(root, "../etc/passwd")
		if err == nil {
			t.Error("expected error for ../etc/passwd")
		}
		_, err = SafePath(root, "sub/../../etc/passwd")
		if err == nil {
			t.Error("expected error for sub/../../etc/passwd")
		}
	})

	t.Run("empty path rejected", func(t *testing.T) {
		_, err := SafePath(root, "")
		if err == nil {
			t.Error("expected error for empty path")
		}
	})
}
