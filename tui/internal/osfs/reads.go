package osfs

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	defaultVariant = "nadir"
	// binaryProbeBytes is how much of a file is checked for NUL bytes.
	binaryProbeBytes = 8 << 10
)

// PathKind classifies p by Lstat; a missing path is KindNone, not an error.
func PathKind(p string) (plan.NodeKind, error) {
	info, err := os.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return plan.KindNone, nil
	case err != nil:
		return plan.KindNone, err
	case info.Mode()&fs.ModeSymlink != 0:
		return plan.KindLink, nil
	case info.IsDir():
		return plan.KindDir, nil
	}
	return plan.KindFile, nil
}

// ReadSide reads p for the resolver without following links. Content is read only
// for a regular file within cfgsync.MaxEditBytes and stays nil when it is binary.
func ReadSide(p string) (cfgsync.Side, error) {
	info, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return cfgsync.Side{}, nil
	}
	if err != nil {
		return cfgsync.Side{}, err
	}
	side := cfgsync.Side{Exists: true, Size: info.Size(), Mode: info.Mode().Perm()}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		side.Kind = plan.KindLink
		side.LinkTarget, err = os.Readlink(p)
		return side, err
	case info.IsDir():
		side.Kind = plan.KindDir
		return side, nil
	}
	side.Kind = plan.KindFile
	if side.Size > cfgsync.MaxEditBytes {
		side.TooLarge = true
		return side, nil
	}
	content, err := os.ReadFile(p)
	if err != nil {
		return side, err
	}
	if bytes.IndexByte(content[:min(len(content), binaryProbeBytes)], 0) >= 0 || !utf8.Valid(content) {
		side.Binary = true
		return side, nil
	}
	side.Content = content
	return side, nil
}

// ReadVariant returns the theme variant, "nadir" unless the file says "zenit".
func ReadVariant(configDir string) string {
	data, err := os.ReadFile(filepath.Join(configDir, "nuydeck", "theme", "variant"))
	if err == nil && strings.TrimSpace(string(data)) == "zenit" {
		return "zenit"
	}
	return defaultVariant
}

// LinkTarget returns the raw target of the symlink at p, without following it.
func LinkTarget(p string) (string, error) { return os.Readlink(p) }

// NewestMTime returns the newest mtime of the files and links under root,
// ignoring paths the filter excludes. Symlinked dirs are not followed; a
// missing root yields the zero time.
func NewestMTime(root string, filter manifest.Filter) (time.Time, error) {
	var newest time.Time
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel != "." && manifest.Excluded(filepath.ToSlash(rel), d.IsDir(), filter) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) && newest.IsZero() {
		return time.Time{}, nil
	}
	return newest, err
}
