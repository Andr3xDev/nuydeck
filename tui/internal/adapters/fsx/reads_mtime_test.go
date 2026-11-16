package fsx

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func TestNewestMTime(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	touch := func(t *testing.T, p string, age time.Duration) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, base.Add(age), base.Add(age)); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("newest wins", func(t *testing.T) {
		root := t.TempDir()
		touch(t, filepath.Join(root, "a"), time.Hour)
		touch(t, filepath.Join(root, "d", "b"), 2*time.Hour)
		got, err := NewestMTime(root, manifest.Filter{})
		if err != nil || !got.Equal(base.Add(2*time.Hour)) {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("excluded ignored", func(t *testing.T) {
		root := t.TempDir()
		touch(t, filepath.Join(root, "a"), time.Hour)
		touch(t, filepath.Join(root, "cache", "x"), 5*time.Hour)
		touch(t, filepath.Join(root, "d", "y.log"), 6*time.Hour)
		got, err := NewestMTime(root, manifest.Filter{Excludes: []string{"/cache", "/d/*.log"}})
		if err != nil || !got.Equal(base.Add(time.Hour)) {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("symlinked dir not followed", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		touch(t, filepath.Join(outside, "new"), 9*time.Hour)
		touch(t, filepath.Join(root, "a"), time.Hour)
		if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		got, err := NewestMTime(root, manifest.Filter{})
		if err != nil || got.Equal(base.Add(9*time.Hour)) || got.Before(base.Add(time.Hour)) {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("missing root", func(t *testing.T) {
		got, err := NewestMTime(filepath.Join(t.TempDir(), "nope"), manifest.Filter{})
		if err != nil || !got.IsZero() {
			t.Fatalf("got %v, %v", got, err)
		}
	})
}
