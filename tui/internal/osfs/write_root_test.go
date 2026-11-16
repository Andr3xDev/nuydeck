package osfs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestWriteFile_WithRootRefusesPathsThatEscapeIt(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	writeFile(t, filepath.Join(root, "keep"), "x")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path string
		leak string
	}{
		{"outside root", filepath.Join(outside, "f"), filepath.Join(outside, "f")},
		{"dot-dot traversal", filepath.Join(root, "..", "outside", "f"), filepath.Join(outside, "f")},
		{"symlinked parent", filepath.Join(root, "linked", "f"), filepath.Join(outside, "f")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := run(t, plan.WriteFileStep{Path: tt.path, Content: []byte("x"), Mode: 0o644, Root: root})
			if res.Status != plan.StepFailed || res.Err == nil {
				t.Fatalf("result = %+v, want refusal", res)
			}
			if exists(tt.leak) {
				t.Fatalf("%s was written", tt.leak)
			}
		})
	}
}

func TestWriteFile_WithRootWritesTheCleanedPathNotTheRawOne(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}

	run(t, plan.WriteFileStep{Path: root + "/linked/../f", Content: []byte("x"), Mode: 0o644, Root: root})

	if exists(filepath.Join(base, "f")) {
		t.Fatal("wrote outside root through symlink/..")
	}
}

func TestWriteFile_WithRootWritesInsideRootAndReplacesSymlinkAtPath(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	victim := filepath.Join(base, "victim")
	writeFile(t, victim, "untouched")
	path := filepath.Join(root, "sub", "f")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, path); err != nil {
		t.Fatal(err)
	}

	res := run(t, plan.WriteFileStep{Path: path, Content: []byte("new"), Mode: 0o644, Root: root})

	if res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("symlink at path survived: %v, %v", info, err)
	}
	if got := readString(t, path); got != "new" {
		t.Errorf("content = %q", got)
	}
	if got := readString(t, victim); got != "untouched" {
		t.Errorf("symlink target was written through: %q", got)
	}
}

func TestWriteFile_WithoutRootDoesNotConfinePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "any", "where")
	if res := run(t, plan.WriteFileStep{Path: path, Content: []byte("x"), Mode: 0o644}); res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if got := readString(t, path); got != "x" {
		t.Errorf("content = %q", got)
	}
}
