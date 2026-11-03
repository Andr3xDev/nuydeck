package domain_test

import (
	"errors"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const testHome = "/home/test"

func manifestWith(entries ...domain.Entry) domain.Manifest {
	return domain.Manifest{
		Version: 1,
		Roots: map[domain.Root]string{
			domain.RootConfig: testHome + "/.config",
			domain.RootHome:   testHome,
			domain.RootFonts:  testHome + "/.local/share/fonts",
			domain.RootThemes: testHome + "/.themes",
		},
		Entries: entries,
	}
}

func entry(root domain.Root, path string) domain.Entry {
	return domain.Entry{Root: root, Path: path}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		m    domain.Manifest
		want error
	}{
		{"valid mixed roots", manifestWith(
			entry("config", "hypr"), entry("config", "starship"), entry("config", "starship.toml"),
			entry("home", ".zshrc"), entry("fonts", "Geist"), entry("themes", "Abysal-Obsidian")), nil},
		{"siblings sharing a name prefix are not nested", manifestWith(
			entry("config", "nvim"), entry("config", "nvim-extra")), nil},
		{"same path in different roots is not a duplicate", manifestWith(
			entry("config", "x"), entry("fonts", "x")), nil},
		{"version 2", func() domain.Manifest { m := manifestWith(entry("config", "hypr")); m.Version = 2; return m }(), domain.ErrUnsupportedVersion},
		{"version missing", func() domain.Manifest { m := manifestWith(entry("config", "hypr")); m.Version = 0; return m }(), domain.ErrUnsupportedVersion},
		{"root outside whitelist", manifestWith(entry("etc", "hosts")), domain.ErrUnknownRoot},
		{"empty root", manifestWith(entry("", "hypr")), domain.ErrUnknownRoot},
		{"unknown root key in [roots]", func() domain.Manifest {
			m := manifestWith(entry("config", "hypr"))
			m.Roots["etc"] = "/etc"
			return m
		}(), domain.ErrUnknownRoot},
		{"empty path", manifestWith(entry("config", "")), domain.ErrInvalidPath},
		{"absolute path", manifestWith(entry("config", "/hypr")), domain.ErrInvalidPath},
		{"parent traversal", manifestWith(entry("config", "../.ssh")), domain.ErrInvalidPath},
		{"embedded parent traversal", manifestWith(entry("config", "hypr/../.ssh")), domain.ErrInvalidPath},
		{"dot path would manage the whole root", manifestWith(entry("home", ".")), domain.ErrInvalidPath},
		{"unclean path", manifestWith(entry("config", "hypr//configs")), domain.ErrInvalidPath},
		{"trailing slash", manifestWith(entry("config", "hypr/")), domain.ErrInvalidPath},
		{"star glob", manifestWith(entry("config", "hypr/*.lua")), domain.ErrInvalidPath},
		{"question glob", manifestWith(entry("config", "hypr/?")), domain.ErrInvalidPath},
		{"bracket glob", manifestWith(entry("config", "hypr/[ab]")), domain.ErrInvalidPath},
		{"duplicate root and path", manifestWith(
			entry("config", "hypr"), entry("home", ".zshrc"), entry("config", "hypr")), domain.ErrDuplicateEntry},
		{"entry nested in another of the same root", manifestWith(
			entry("config", "hypr"), entry("config", "hypr/configs")), domain.ErrNestedEntry},
		{"parent declared after child", manifestWith(
			entry("config", "hypr/configs"), entry("config", "hypr")), domain.ErrNestedEntry},
		{"entry is the backups dir", manifestWith(entry("home", ".nuydeck-backups")), domain.ErrInsideBackups},
		{"entry inside the backups dir", manifestWith(entry("home", ".nuydeck-backups/20260101-000000")), domain.ErrInsideBackups},
		{"root resolves into the backups dir", func() domain.Manifest {
			m := manifestWith(entry("fonts", "latest"))
			m.Roots[domain.RootFonts] = testHome + "/.nuydeck-backups"
			return m
		}(), domain.ErrInsideBackups},
		{"similarly named dir is not the backups dir", manifestWith(entry("home", ".nuydeck-backups-old")), nil},
		{"entry is ~/.secrets", manifestWith(entry("home", ".secrets")), domain.ErrSecretsPath},
		{"entry inside ~/.secrets", manifestWith(entry("home", ".secrets/token")), domain.ErrSecretsPath},
		{"root resolves to ~/.secrets", func() domain.Manifest {
			m := manifestWith(entry("themes", "x"))
			m.Roots[domain.RootThemes] = testHome + "/.secrets"
			return m
		}(), domain.ErrSecretsPath},
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
