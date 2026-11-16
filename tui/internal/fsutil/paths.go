package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Within reports whether p is base or lies below it, lexically.
func Within(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func StrictlyWithin(base, p string) bool {
	return Within(base, p) && filepath.Clean(base) != filepath.Clean(p)
}

// ResolveExisting resolves symlinks in the deepest existing ancestor of p and
// re-appends the missing tail, so paths that do not exist yet can be compared.
func ResolveExisting(p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	if !errors.Is(err, fs.ErrNotExist) {
		return resolved, err
	}
	// EvalSymlinks also reports ErrNotExist for a dangling link, whose target could be anywhere.
	if _, statErr := os.Lstat(p); statErr == nil {
		return "", fmt.Errorf("cannot resolve dangling symlink %s", p)
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p, nil
	}
	resolvedParent, err := ResolveExisting(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(p)), nil
}
