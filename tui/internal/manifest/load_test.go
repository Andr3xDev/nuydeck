package manifest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

const home = "/home/test"

func TestLoadManifestExpandsTilde(t *testing.T) {
	m, err := manifest.LoadManifest("testdata/valid/paths.toml", home)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	want := map[manifest.Root]string{manifest.RootConfig: home + "/.config", manifest.RootHome: home}
	for root, dest := range want {
		if m.Roots[root] != dest {
			t.Errorf("Roots[%s] = %q, want %q", root, m.Roots[root], dest)
		}
	}
	if len(m.Entries) != 2 || len(m.Excludes) != 1 {
		t.Errorf("got %d entries and %d excludes, want 2 and 1", len(m.Entries), len(m.Excludes))
	}
}

func TestLoadManifestErrorsNameFileAndLine(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		wantLine int
		wantText []string
	}{
		{"syntax error", "syntax", 3, nil},
		{"unknown key", "unknown_key", 9, []string{"colour"}},
		{"missing required field", "missing_field", 11, []string{"path"}},
		{"duplicate names both entries", "duplicate", 15, []string{"config", "hypr", "line 7"}},
		{"headers with inner spaces are located", "spaced_header", 15, []string{"config", "hypr", "line 7"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifest.LoadManifest("testdata/"+tt.fixture+"/paths.toml", home)
			if err == nil {
				t.Fatal("LoadManifest() error = nil, want error")
			}
			if prefix := fmt.Sprintf("paths.toml:%d: ", tt.wantLine); !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("error = %q, want prefix %q", err, prefix)
			}
			for _, text := range tt.wantText {
				if !strings.Contains(err.Error(), text) {
					t.Errorf("error = %q, want it to contain %q", err, text)
				}
			}
		})
	}
}

// Inline arrays have no [[entry]] header to map an entry index to.
func TestLoadManifestValidationErrorWithoutHeaderLinesDoesNotPanic(t *testing.T) {
	_, err := manifest.LoadManifest("testdata/inline_entries/paths.toml", home)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
	if !strings.HasPrefix(err.Error(), "paths.toml:") || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want paths.toml: prefix and duplicate", err)
	}
}

func TestLoadManifestMissingFile(t *testing.T) {
	if _, err := manifest.LoadManifest("testdata/absent/paths.toml", home); err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
}

// Guards the repo manifest's invariants without pinning its full inventory, so adding a path stays free.
func TestRepoManifestKeepsSpecInvariants(t *testing.T) {
	m, err := manifest.LoadManifest("../../../manifest/paths.toml", home)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	requiredEntries := []string{"config/hypr", "config/nvim", "config/quickshell/lucyna", "home/.zshrc"}
	requiredExcludes := []string{
		"config/hypr/configs/monitors_generated.lua",
		"config/quickshell/lucyna/legacy/monitors/data/identities.json",
		"config/systemd/user/*.wants/", "config/nuydeck/logs/", "config/nuydeck/theme/", "config/nvim/lazy-lock.json",
	}

	gotEntries := map[string]bool{}
	for _, e := range m.Entries {
		key := string(e.Root) + "/" + e.Path
		gotEntries[key] = true
		for _, banned := range []string{"spicetify", "Inter", ".secrets", ".nuydeck-backups"} {
			if strings.Contains(key, banned) {
				t.Errorf("entry %q must not be managed (contains %q)", key, banned)
			}
		}
	}
	if gotEntries["config/nvim/lazy-lock.json"] {
		t.Error("nvim/lazy-lock.json must be an exclude, not an entry")
	}
	gotExcludes := map[string]bool{}
	for _, x := range m.Excludes {
		gotExcludes[string(x.Root)+"/"+x.Pattern] = true
	}

	for _, want := range requiredEntries {
		if !gotEntries[want] {
			t.Errorf("missing entry %q", want)
		}
	}
	for _, want := range requiredExcludes {
		if !gotExcludes[want] {
			t.Errorf("missing exclude %q", want)
		}
	}
}
