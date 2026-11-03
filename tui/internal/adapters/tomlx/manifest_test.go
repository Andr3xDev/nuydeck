package tomlx_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/tomlx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const home = "/home/test"

func TestLoadManifestExpandsTilde(t *testing.T) {
	m, err := tomlx.LoadManifest("testdata/valid/paths.toml", home)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	want := map[domain.Root]string{domain.RootConfig: home + "/.config", domain.RootHome: home}
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tomlx.LoadManifest("testdata/"+tt.fixture+"/paths.toml", home)
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

func TestLoadManifestMissingFile(t *testing.T) {
	if _, err := tomlx.LoadManifest("testdata/absent/paths.toml", home); err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
}

// Guards the repo manifest against drifting from the spec inventory (tasks 1.3 and 2.4).
func TestRepoManifestMatchesSpecInventory(t *testing.T) {
	m, err := tomlx.LoadManifest("../../../../manifest/paths.toml", home)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	wantEntries := []string{
		"config/awww", "config/btop", "config/dunst", "config/fastfetch", "config/ghostty", "config/gtk-3.0",
		"config/hypr", "config/kotofetch", "config/lazygit", "config/nvim", "config/nwg-look",
		"config/quickshell/lucyna", "config/satty", "config/starship", "config/starship.toml", "config/yazi",
		"config/wallpapers", "config/systemd/user/lucyna-clipboard-wipe.service",
		"config/systemd/user/lucyna-clipboard-wipe.timer", "config/nuydeck/lib", "config/nuydeck/scripts",
		"config/git/ignore", "config/mimeapps.list", "config/openspec/config.json", "config/Vial/Vial.conf",
		"home/.zshrc", "home/.gtkrc-2.0", "home/.icons/default/index.theme",
		"home/.local/bin/claude-out-context", "home/.local/bin/codegraph-health",
		"fonts/Geist", "fonts/soulmaze", "themes/Abysal-Obsidian", "themes/Abysal-Marble",
	}
	wantExcludes := []string{
		"config/hypr/configs/monitors_generated.lua",
		"config/quickshell/lucyna/legacy/monitors/data/identities.json",
		"config/systemd/user/*.wants/", "config/nuydeck/logs/", "config/nuydeck/theme/", "config/nvim/lazy-lock.json",
	}

	gotEntries := map[string]bool{}
	for _, e := range m.Entries {
		key := string(e.Root) + "/" + e.Path
		gotEntries[key] = true
		for _, banned := range []string{"spicetify", "Inter", ".secrets", ".env"} {
			if strings.Contains(key, banned) {
				t.Errorf("entry %q must not be managed (contains %q)", key, banned)
			}
		}
	}
	gotExcludes := map[string]bool{}
	for _, x := range m.Excludes {
		gotExcludes[string(x.Root)+"/"+x.Pattern] = true
	}

	for _, want := range wantEntries {
		if !gotEntries[want] {
			t.Errorf("missing entry %q", want)
		}
	}
	for _, want := range wantExcludes {
		if !gotExcludes[want] {
			t.Errorf("missing exclude %q", want)
		}
	}
	if len(m.Entries) != len(wantEntries) || len(m.Excludes) != len(wantExcludes) {
		t.Errorf("got %d entries and %d excludes, want exactly %d and %d",
			len(m.Entries), len(m.Excludes), len(wantEntries), len(wantExcludes))
	}
}
