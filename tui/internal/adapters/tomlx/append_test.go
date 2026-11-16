package tomlx_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/tomlx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const existingCatalog = `# curated by hand
version = 1

[[package]]
name = "hyprland"   # keep first
source = "pacman"
`

func TestAppendPackage_KeepsExistingTextAndAddsLoadableBlock(t *testing.T) {
	for _, tt := range []struct {
		name     string
		existing string
	}{
		{"with trailing newline", existingCatalog},
		{"without trailing newline", strings.TrimSuffix(existingCatalog, "\n")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, "packages.toml", tt.existing)
			if err := os.Chmod(path, 0o640); err != nil {
				t.Fatal(err)
			}
			entry := domain.Package{Name: "htop", Source: domain.SourceAUR, Profiles: []string{"full"}, Group: "tools"}
			if err := tomlx.AppendPackage(path, entry); err != nil {
				t.Fatalf("AppendPackage() error = %v", err)
			}
			if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
				t.Errorf("mode = %o, want 640", info.Mode().Perm())
			}
			raw, _ := os.ReadFile(path)
			if !strings.HasPrefix(string(raw), tt.existing) {
				t.Errorf("existing text was not preserved:\n%s", raw)
			}
			got, err := tomlx.LoadPackages(path)
			if err != nil {
				t.Fatalf("LoadPackages() after append: %v", err)
			}
			if len(got) != 2 || got[1].Name != "htop" || got[1].Source != domain.SourceAUR ||
				got[1].Group != "tools" || !reflect.DeepEqual(got[1].Profiles, []string{"full"}) {
				t.Errorf("packages = %+v, want hyprland then htop", got)
			}
			if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
				t.Errorf("leftover temp files: %v", entries)
			}
		})
	}
}

func TestAppendPackage_RejectedEntryLeavesFileUntouched(t *testing.T) {
	tests := []struct {
		name  string
		entry domain.Package
	}{
		{"duplicate name", domain.Package{Name: "hyprland", Source: domain.SourcePacman}},
		{"missing source", domain.Package{Name: "htop"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, "packages.toml", existingCatalog)
			if err := tomlx.AppendPackage(path, tt.entry); err == nil {
				t.Fatal("AppendPackage() error = nil, want error")
			}
			raw, _ := os.ReadFile(path)
			if string(raw) != existingCatalog {
				t.Errorf("file changed:\n%s", raw)
			}
			if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
				t.Errorf("leftover temp files: %v", entries)
			}
		})
	}
}
