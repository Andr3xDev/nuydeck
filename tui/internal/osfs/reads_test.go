package osfs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestPathKind_MissingPathIsKindNoneNotError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f"), "x")
	_ = os.Mkdir(filepath.Join(dir, "d"), 0o755)
	_ = os.Symlink("f", filepath.Join(dir, "l"))

	for name, want := range map[string]plan.NodeKind{
		"f": plan.KindFile, "d": plan.KindDir, "l": plan.KindLink, "nope": plan.KindNone,
	} {
		got, err := osfs.PathKind(filepath.Join(dir, name))
		if err != nil || got != want {
			t.Errorf("PathKind(%s) = %v, %v; want %v, nil", name, got, err, want)
		}
	}
}

func TestReadVariant_TrimsAndFallsBackToNadir(t *testing.T) {
	for name, content := range map[string]*string{
		"zenit trimmed": ptr("  zenit\n"),
		"nadir":         ptr("nadir\n"),
		"other value":   ptr("purple"),
		"missing file":  nil,
	} {
		cfg := t.TempDir()
		if content != nil {
			writeFile(t, filepath.Join(cfg, "nuydeck", "theme", "variant"), *content)
		}
		want := "nadir"
		if name == "zenit trimmed" {
			want = "zenit"
		}
		if got := osfs.ReadVariant(cfg); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

func ptr(s string) *string { return &s }
