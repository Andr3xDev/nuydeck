package osfs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

const (
	privateDirMode     = fs.FileMode(0o700)
	groupWorldWritable = fs.FileMode(0o022)
	tempDirPattern     = "nuydeck-edit-*"
)

// CheckPrivateDir refuses a directory another user could swap files in.
func CheckPrivateDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Geteuid() {
		return fmt.Errorf("%s is not owned by the current user", dir)
	}
	if info.Mode().Perm()&groupWorldWritable != 0 {
		return fmt.Errorf("%s is writable by group or others", dir)
	}
	return nil
}

// MkdirPrivate creates a 0700 directory under XDG_RUNTIME_DIR, else ~/.cache/nuydeck/tmp.
func MkdirPrivate() (string, error) {
	parent := os.Getenv("XDG_RUNTIME_DIR")
	if parent == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		parent = filepath.Join(home, ".cache", "nuydeck", "tmp")
		if err := os.MkdirAll(parent, privateDirMode); err != nil {
			return "", err
		}
	}
	if err := CheckPrivateDir(parent); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, tempDirPattern)
}

// TempFS is the real cfgsync.TempFS.
type TempFS struct{}

func (TempFS) MkdirPrivate() (string, error)        { return MkdirPrivate() }
func (TempFS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (TempFS) RemoveAll(path string) error          { return os.RemoveAll(path) }
func (TempFS) WriteFile(path string, data []byte, mode fs.FileMode) error {
	return os.WriteFile(path, data, mode)
}
