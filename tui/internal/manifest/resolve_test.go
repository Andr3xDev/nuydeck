package manifest_test

import (
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func TestResolve(t *testing.T) {
	m := manifest.Manifest{
		Roots: map[manifest.Root]string{manifest.RootConfig: "/home/u/.config", manifest.RootHome: "/home/u"},
		Entries: []manifest.Entry{
			{Root: manifest.RootConfig, Path: "hypr"},
			{Root: manifest.RootConfig, Path: "nvim/lazy-lock.json"},
			{Root: manifest.RootHome, Path: ".zshrc"},
		},
		Excludes: []manifest.Exclude{
			{Root: manifest.RootConfig, Pattern: "hypr/configs/generated.lua"},
			{Root: manifest.RootConfig, Pattern: "nvim/lazy-lock.json"},
		},
	}

	got := manifest.Resolve(m, "/repo")

	want := []manifest.ResolvedEntry{
		{
			Entry:    manifest.Entry{Root: manifest.RootConfig, Path: "hypr"},
			RepoPath: "/repo/config/hypr",
			DestPath: "/home/u/.config/hypr",
			Filter:   manifest.Filter{Excludes: []string{"/configs/generated.lua"}},
		},
		{
			Entry:    manifest.Entry{Root: manifest.RootHome, Path: ".zshrc"},
			RepoPath: "/repo/home/.zshrc",
			DestPath: "/home/u/.zshrc",
		},
	}
	if !slices.EqualFunc(got, want, func(a, b manifest.ResolvedEntry) bool {
		return a.Entry == b.Entry && a.RepoPath == b.RepoPath && a.DestPath == b.DestPath && slices.Equal(a.Filter.Excludes, b.Filter.Excludes)
	}) {
		t.Errorf("Resolve = %+v, want %+v (whole-excluded entry dropped, order kept, filters anchored)", got, want)
	}
}

func TestResolve_EmptyManifestGivesNoEntries(t *testing.T) {
	if got := manifest.Resolve(manifest.Manifest{}, "/repo"); len(got) != 0 {
		t.Errorf("Resolve = %+v, want none", got)
	}
}
