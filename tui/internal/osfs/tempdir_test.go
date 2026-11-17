package osfs_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
)

func assertPrivateDir(t *testing.T, dir string) {
	t.Helper()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Errorf("%s mode = %v, want directory 0700", dir, info.Mode())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Geteuid() {
		t.Errorf("%s is not owned by the current user", dir)
	}
}

func TestMkdirPrivate_UsesRuntimeDirWhenSet(t *testing.T) {
	runtime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtime)

	dir, err := osfs.MkdirPrivate()

	if err != nil {
		t.Fatalf("MkdirPrivate: %v", err)
	}
	if filepath.Dir(dir) != runtime {
		t.Errorf("dir = %s, want a child of %s", dir, runtime)
	}
	assertPrivateDir(t, dir)
}

func TestMkdirPrivate_FallsBackToCacheUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("HOME", home)

	dir, err := osfs.MkdirPrivate()

	if err != nil {
		t.Fatalf("MkdirPrivate: %v", err)
	}
	cache := filepath.Join(home, ".cache", "nuydeck", "tmp")
	if filepath.Dir(dir) != cache {
		t.Errorf("dir = %s, want a child of %s", dir, cache)
	}
	assertPrivateDir(t, dir)
	assertPrivateDir(t, cache)
}

func TestMkdirPrivate_ReturnsDistinctDirs(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	a, errA := osfs.MkdirPrivate()
	b, errB := osfs.MkdirPrivate()

	if errA != nil || errB != nil || a == b {
		t.Errorf("dirs = %q, %q (errs %v, %v), want two distinct dirs", a, b, errA, errB)
	}
}

func TestMkdirPrivate_RefusesRuntimeDirOthersCanWrite(t *testing.T) {
	runtime := t.TempDir()
	if err := os.Chmod(runtime, 0o770); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", runtime)

	dir, err := osfs.MkdirPrivate()

	if err == nil || !strings.Contains(err.Error(), "writable") {
		t.Errorf("MkdirPrivate = %q, %v, want a writable-by-others error", dir, err)
	}
}
