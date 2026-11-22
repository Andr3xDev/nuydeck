package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRepoOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "manifest"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, manifestRelPath), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, "manifest"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, manifestRelPath), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cwdAbs, err := filepath.Abs("rel")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, flagRepo, stateRepo, cwd, want string
	}{
		{"flag wins", "/f", "/s", deep, "/f"},
		{"flag made absolute", "rel", "", deep, cwdAbs},
		{"state next", "", other, deep, other},
		{"stale state falls to walk up", "", "/gone", deep, root},
		{"walk up", "", "", deep, root},
		{"none", "", "", t.TempDir(), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveRepo(tt.flagRepo, tt.stateRepo, tt.cwd); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
