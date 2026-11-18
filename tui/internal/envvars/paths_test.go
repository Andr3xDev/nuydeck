package envvars_test

import (
	"errors"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/envvars"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func hyprEntry() manifest.ResolvedEntry {
	return manifest.ResolvedEntry{
		Entry:    manifest.Entry{Root: manifest.RootConfig, Path: "hypr"},
		RepoPath: "/repo/config/hypr",
		DestPath: "/home/u/.config/hypr",
	}
}

func TestPaths_JoinsEnvFileOntoHyprEntry(t *testing.T) {
	other := manifest.ResolvedEntry{
		Entry:    manifest.Entry{Root: manifest.RootConfig, Path: "waybar"},
		RepoPath: "/repo/config/waybar",
		DestPath: "/home/u/.config/waybar",
	}
	got, err := envvars.Paths([]manifest.ResolvedEntry{other, hyprEntry()})
	if err != nil {
		t.Fatalf("Paths() error = %v", err)
	}
	if got.RepoPath != "/repo/config/hypr/configs/env_variables.lua" {
		t.Errorf("RepoPath = %q", got.RepoPath)
	}
	if got.DestPath != "/home/u/.config/hypr/configs/env_variables.lua" {
		t.Errorf("DestPath = %q", got.DestPath)
	}
}

func TestPaths_WithoutHyprEntryReturnsErrNoHyprEntry(t *testing.T) {
	home := manifest.ResolvedEntry{Entry: manifest.Entry{Root: manifest.RootHome, Path: "hypr"}, RepoPath: "/repo/home/hypr", DestPath: "/home/u/hypr"}
	for name, entries := range map[string][]manifest.ResolvedEntry{"nil": nil, "same path other root": {home}} {
		t.Run(name, func(t *testing.T) {
			if _, err := envvars.Paths(entries); !errors.Is(err, envvars.ErrNoHyprEntry) {
				t.Errorf("Paths() error = %v, want ErrNoHyprEntry", err)
			}
		})
	}
}

func TestLocations_PathPicksCopyBySource(t *testing.T) {
	loc, err := envvars.Paths([]manifest.ResolvedEntry{hyprEntry()})
	if err != nil {
		t.Fatal(err)
	}
	if got := loc.Path(envvars.SourceRepo); got != loc.RepoPath {
		t.Errorf("Path(repo) = %q, want %q", got, loc.RepoPath)
	}
	if got := loc.Path(envvars.SourceLive); got != loc.DestPath {
		t.Errorf("Path(live) = %q, want %q", got, loc.DestPath)
	}
}
