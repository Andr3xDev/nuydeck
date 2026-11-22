package manifest_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

const testHome = "/home/test"

func manifestWith(entries ...manifest.Entry) manifest.Manifest {
	return manifest.Manifest{
		Version: 1,
		Roots: map[manifest.Root]string{
			manifest.RootConfig: testHome + "/.config",
			manifest.RootHome:   testHome,
			manifest.RootFonts:  testHome + "/.local/share/fonts",
			manifest.RootThemes: testHome + "/.themes",
		},
		Entries: entries,
	}
}

func entry(root manifest.Root, path string) manifest.Entry {
	return manifest.Entry{Root: root, Path: path}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		m    manifest.Manifest
		want error
	}{
		{"valid mixed roots", manifestWith(
			entry("config", "hypr"), entry("config", "starship"), entry("config", "starship.toml"),
			entry("home", ".zshrc"), entry("fonts", "Geist"), entry("themes", "Abysal-Obsidian")), nil},
		{"siblings sharing a name prefix are not nested", manifestWith(
			entry("config", "nvim"), entry("config", "nvim-extra")), nil},
		{"same path in different roots is not a duplicate", manifestWith(
			entry("config", "x"), entry("fonts", "x")), nil},
		{"version 2", func() manifest.Manifest { m := manifestWith(entry("config", "hypr")); m.Version = 2; return m }(), manifest.ErrUnsupportedVersion},
		{"version missing", func() manifest.Manifest { m := manifestWith(entry("config", "hypr")); m.Version = 0; return m }(), manifest.ErrUnsupportedVersion},
		{"root outside whitelist", manifestWith(entry("etc", "hosts")), manifest.ErrUnknownRoot},
		{"empty root", manifestWith(entry("", "hypr")), manifest.ErrUnknownRoot},
		{"unknown root key in [roots]", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Roots["etc"] = "/etc"
			return m
		}(), manifest.ErrUnknownRoot},
		{"entry root missing from [roots]", func() manifest.Manifest {
			m := manifestWith(entry("themes", "x"))
			delete(m.Roots, manifest.RootThemes)
			return m
		}(), manifest.ErrUnknownRoot},
		{"relative root destination", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Roots[manifest.RootConfig] = ".config"
			return m
		}(), manifest.ErrInvalidRoot},
		{"empty root destination", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Roots[manifest.RootConfig] = ""
			return m
		}(), manifest.ErrInvalidRoot},
		{"unused root with relative destination", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Roots[manifest.RootFonts] = "fonts"
			return m
		}(), manifest.ErrInvalidRoot},
		{"exclude on a root absent from [roots]", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			delete(m.Roots, manifest.RootThemes)
			m.Excludes = []manifest.Exclude{{Root: "themes", Pattern: "x"}}
			return m
		}(), manifest.ErrInvalidExclude},
		{"exclude on an unknown root", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Excludes = []manifest.Exclude{{Root: "etc", Pattern: "x"}}
			return m
		}(), manifest.ErrInvalidExclude},
		{"exclude with empty pattern", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Excludes = []manifest.Exclude{{Root: "config", Pattern: ""}}
			return m
		}(), manifest.ErrInvalidExclude},
		{"exclude with absolute pattern", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Excludes = []manifest.Exclude{{Root: "config", Pattern: "/hypr/x"}}
			return m
		}(), manifest.ErrInvalidExclude},
		{"exclude with parent traversal", func() manifest.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Excludes = []manifest.Exclude{{Root: "config", Pattern: "hypr/../.ssh"}}
			return m
		}(), manifest.ErrInvalidExclude},
		{"exclude with glob and trailing slash is valid", func() manifest.Manifest {
			m := manifestWith(entry("config", "systemd/user/x"))
			m.Excludes = []manifest.Exclude{{Root: "config", Pattern: "systemd/user/*.wants/"}}
			return m
		}(), nil},
		{"empty path", manifestWith(entry("config", "")), manifest.ErrInvalidPath},
		{"absolute path", manifestWith(entry("config", "/hypr")), manifest.ErrInvalidPath},
		{"parent traversal", manifestWith(entry("config", "../.ssh")), manifest.ErrInvalidPath},
		{"embedded parent traversal", manifestWith(entry("config", "hypr/../.ssh")), manifest.ErrInvalidPath},
		{"dot path would manage the whole root", manifestWith(entry("home", ".")), manifest.ErrInvalidPath},
		{"unclean path", manifestWith(entry("config", "hypr//configs")), manifest.ErrInvalidPath},
		{"trailing slash", manifestWith(entry("config", "hypr/")), manifest.ErrInvalidPath},
		{"star glob", manifestWith(entry("config", "hypr/*.lua")), manifest.ErrInvalidPath},
		{"question glob", manifestWith(entry("config", "hypr/?")), manifest.ErrInvalidPath},
		{"bracket glob", manifestWith(entry("config", "hypr/[ab]")), manifest.ErrInvalidPath},
		{"duplicate root and path", manifestWith(
			entry("config", "hypr"), entry("home", ".zshrc"), entry("config", "hypr")), manifest.ErrDuplicateEntry},
		{"entry nested in another of the same root", manifestWith(
			entry("config", "hypr"), entry("config", "hypr/configs")), manifest.ErrNestedEntry},
		{"parent declared after child", manifestWith(
			entry("config", "hypr/configs"), entry("config", "hypr")), manifest.ErrNestedEntry},
		{"entry is the backups dir", manifestWith(entry("home", ".nuydeck-backups")), manifest.ErrInsideBackups},
		{"entry inside the backups dir", manifestWith(entry("home", ".nuydeck-backups/20260101-000000")), manifest.ErrInsideBackups},
		{"root resolves into the backups dir", func() manifest.Manifest {
			m := manifestWith(entry("fonts", "latest"))
			m.Roots[manifest.RootFonts] = testHome + "/.nuydeck-backups"
			return m
		}(), manifest.ErrInsideBackups},
		{"similarly named dir is not the backups dir", manifestWith(entry("home", ".nuydeck-backups-old")), nil},
		{"entry is ~/.secrets", manifestWith(entry("home", ".secrets")), manifest.ErrSecretsPath},
		{"entry inside ~/.secrets", manifestWith(entry("home", ".secrets/token")), manifest.ErrSecretsPath},
		{"root resolves to ~/.secrets", func() manifest.Manifest {
			m := manifestWith(entry("themes", "x"))
			m.Roots[manifest.RootThemes] = testHome + "/.secrets"
			return m
		}(), manifest.ErrSecretsPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate(testHome)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() = %v, want errors.Is %v", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsEntryThatContainsSecretsOrBackups(t *testing.T) {
	tests := []struct {
		name string
		root string
		path string
		ok   bool
	}{
		{"entry is the home dir", "/home", "test", false},
		{"entry is an ancestor of home", "/", "home", false},
		{"entry is a sibling of the home dir", "/home", "test2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := manifestWith(entry("themes", tt.path))
			m.Roots[manifest.RootThemes] = tt.root
			err := m.Validate(testHome)
			if tt.ok {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, manifest.ErrSecretsPath) && !errors.Is(err, manifest.ErrInsideBackups) {
				t.Fatalf("Validate() = %v, want ErrSecretsPath or ErrInsideBackups", err)
			}
		})
	}
}

func TestValidateReportsSmallestUnknownRootKey(t *testing.T) {
	// Map iteration order is random; repeat so a nondeterministic pick fails reliably.
	for range 30 {
		m := manifestWith(entry("config", "hypr"))
		m.Roots["zzz"] = "/z"
		m.Roots["etc"] = "/etc"
		err := m.Validate(testHome)
		if !errors.Is(err, manifest.ErrUnknownRoot) || !strings.Contains(err.Error(), `"etc"`) {
			t.Fatalf("Validate() = %v, want ErrUnknownRoot naming \"etc\"", err)
		}
	}
}

func TestValidateEntryErrorCarriesIndexAndPrior(t *testing.T) {
	tests := []struct {
		name      string
		m         manifest.Manifest
		wantIndex int
		wantPrior int
	}{
		{"duplicate", manifestWith(entry("config", "hypr"), entry("home", ".zshrc"), entry("config", "hypr")), 2, 0},
		{"child after parent", manifestWith(entry("config", "hypr"), entry("config", "hypr/configs")), 1, 0},
		{"parent after child", manifestWith(entry("home", ".zshrc"), entry("config", "hypr/configs"), entry("config", "hypr")), 2, 1},
		{"no prior for a path error", manifestWith(entry("config", "hypr"), entry("config", "")), 1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entryErr *manifest.EntryError
			if !errors.As(tt.m.Validate(testHome), &entryErr) {
				t.Fatal("Validate() is not an *EntryError")
			}
			if entryErr.Index != tt.wantIndex || entryErr.Prior != tt.wantPrior {
				t.Errorf("Index, Prior = %d, %d, want %d, %d", entryErr.Index, entryErr.Prior, tt.wantIndex, tt.wantPrior)
			}
		})
	}
}
