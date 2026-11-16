package osfs_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestDeleteStep(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(root, outside string) (path string)
		wantRemove bool
		wantFail   bool
		wantDetail string
	}{
		{"file inside root is removed", func(root, _ string) string {
			p := filepath.Join(root, "a.txt")
			writeFile(t, p, "x")
			return p
		}, true, false, ""},
		{"empty dir inside root is removed", func(root, _ string) string {
			p := filepath.Join(root, "d")
			_ = os.Mkdir(p, 0o755)
			return p
		}, true, false, ""},
		{"non-empty dir is kept, not an error", func(root, _ string) string {
			p := filepath.Join(root, "d")
			writeFile(t, filepath.Join(p, "child"), "x")
			return p
		}, false, false, "kept (not empty)"},
		{"symlink is removed and its target survives", func(root, outside string) string {
			target := filepath.Join(outside, "live.txt")
			writeFile(t, target, "live")
			p := filepath.Join(root, "link")
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
			return p
		}, true, false, ""},
		{"path outside root is refused", func(_, outside string) string {
			p := filepath.Join(outside, "victim.txt")
			writeFile(t, p, "x")
			return p
		}, false, true, ""},
		{"dot-dot traversal out of root is refused", func(root, outside string) string {
			writeFile(t, filepath.Join(outside, "victim.txt"), "x")
			return filepath.Join(root, "..", filepath.Base(outside), "victim.txt")
		}, false, true, ""},
		{"sibling sharing the root name prefix is refused", func(root, _ string) string {
			p := root + "-sibling/victim.txt"
			writeFile(t, p, "x")
			return p
		}, false, true, ""},
		{"the root itself is refused", func(root, _ string) string { return root }, false, true, ""},
		{"path below a symlinked parent is refused", func(root, outside string) string {
			writeFile(t, filepath.Join(outside, "victim.txt"), "x")
			if err := os.Symlink(outside, filepath.Join(root, "via")); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(root, "via", "victim.txt")
		}, false, true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "root")
			outside := filepath.Join(base, "outside")
			_ = os.MkdirAll(root, 0o755)
			_ = os.MkdirAll(outside, 0o755)
			path := tc.setup(root, outside)

			res := osfs.Runner{Home: base}.Run(context.Background(), plan.DeleteStep{Path: path, Root: root}, nil)

			if gotFail := res.Status == plan.StepFailed; gotFail != tc.wantFail {
				t.Fatalf("status = %v, err = %v; wantFail %v", res.Status, res.Err, tc.wantFail)
			}
			if tc.wantFail && res.Err == nil {
				t.Fatal("refusal must carry an error")
			}
			if tc.wantDetail != "" && !strings.Contains(res.Detail, tc.wantDetail) {
				t.Fatalf("detail = %q, want %q", res.Detail, tc.wantDetail)
			}
			if removed := !exists(path); tc.wantRemove != removed {
				t.Fatalf("removed = %v, want %v", removed, tc.wantRemove)
			}
			if tc.wantFail && !exists(path) {
				t.Fatal("refused path was deleted")
			}
			if tc.name == "symlink is removed and its target survives" && !exists(filepath.Join(outside, "live.txt")) {
				t.Fatal("link target was deleted")
			}
		})
	}
}
