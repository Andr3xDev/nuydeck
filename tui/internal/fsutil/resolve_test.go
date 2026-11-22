package fsutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/fsutil"
)

func TestResolveExisting_DanglingSymlinkFailsClosed(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "dangling")
	if err := os.Symlink(filepath.Join(dir, "nowhere"), link); err != nil {
		t.Fatal(err)
	}

	for name, p := range map[string]string{
		"link itself":           link,
		"missing tail below it": filepath.Join(link, "sub", "file"),
	} {
		if got, err := fsutil.ResolveExisting(p); err == nil {
			t.Errorf("%s: resolved to %q, want an error", name, got)
		}
	}
	if _, err := fsutil.ResolveExisting(filepath.Join(dir, "plain-missing", "file")); err != nil {
		t.Errorf("plain missing path must still resolve: %v", err)
	}
}
