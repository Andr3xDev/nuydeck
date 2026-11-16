package cfgsync_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func linkEntry(root manifest.Root, path string) manifest.Entry {
	return manifest.Entry{Root: root, Path: path}
}

func TestClassifyLink(t *testing.T) {
	entries := []manifest.Entry{
		linkEntry("config", "starship"), linkEntry("config", "starship.toml"),
		linkEntry("config", "hypr"), linkEntry("config", "nvim"), linkEntry("config", "dunst"),
	}
	tests := []struct {
		name    string
		linkDir string
		target  string
		want    plan.LinkWarning
	}{
		{"absolute target", "", "/usr/share/starship.toml", plan.WarnAbsoluteLink},
		{"absolute target pointing into the managed set", "hypr", "/home/test/.config/hypr/x", plan.WarnAbsoluteLink},
		{"variant link stays inside a sibling entry", "", "starship/nuytek-nadir/starship.toml", plan.NoLinkWarning},
		{"sibling file inside own entry", "dunst", "dunstrc-nadir", plan.NoLinkWarning},
		{"climbs into another managed entry", "hypr", "../nvim/init.lua", plan.NoLinkWarning},
		{"points at an entry itself", "hypr", "../nvim", plan.NoLinkWarning},
		{"resolves to an unmanaged sibling", "hypr", "../btop/btop.conf", plan.WarnLeavesManagedSet},
		{"resolves to unmanaged file at root", "hypr/theme", "../../foo", plan.WarnLeavesManagedSet},
		{"resolves to the root itself", "hypr", "..", plan.WarnLeavesManagedSet},
		{"climbs above the root", "hypr/theme", "../../../.ssh/config", plan.WarnLeavesManagedSet},
		{"unclean relative target is cleaned first", "hypr", "./theme/../colors.conf", plan.NoLinkWarning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfgsync.ClassifyLink("config", tt.linkDir, tt.target, entries); got != tt.want {
				t.Errorf("ClassifyLink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyLinkIgnoresEntriesOfOtherRoots(t *testing.T) {
	entries := []manifest.Entry{linkEntry("home", "nvim")}
	if got := cfgsync.ClassifyLink("config", "hypr", "../nvim/init.lua", entries); got != plan.WarnLeavesManagedSet {
		t.Errorf("ClassifyLink() = %v, want WarnLeavesManagedSet", got)
	}
}

func TestPreflightBlocked(t *testing.T) {
	const dest = "/home/test/.config/hypr"
	kinds := func(m map[string]plan.NodeKind) func(string) (plan.NodeKind, error) {
		return func(p string) (plan.NodeKind, error) { return m[p], nil } // missing path = KindNone
	}
	changes := []plan.Change{
		{Path: "a/b/new.lua", Type: plan.Create},
		{Path: "a/old.lua", Type: plan.Update},
		{Path: "c/other.lua", Type: plan.Update},
	}
	tests := []struct {
		name    string
		srcKind plan.NodeKind
		kinds   map[string]plan.NodeKind
		changes []plan.Change
		want    []string
	}{
		{"real dirs are fine", plan.KindDir, map[string]plan.NodeKind{dest: plan.KindDir, dest + "/a": plan.KindDir, dest + "/c": plan.KindDir}, changes, nil},
		{"missing parents are fine", plan.KindDir, map[string]plan.NodeKind{dest: plan.KindDir}, changes, nil},
		{"destination root symlinked blocks everything", plan.KindDir, map[string]plan.NodeKind{dest: plan.KindLink}, changes,
			[]string{"a/b/new.lua", "a/old.lua", "c/other.lua"}},
		{"symlinked parent blocks only items below it", plan.KindDir, map[string]plan.NodeKind{dest: plan.KindDir, dest + "/a": plan.KindLink}, changes,
			[]string{"a/b/new.lua", "a/old.lua"}},
		{"deep symlinked parent blocks only its subtree", plan.KindDir, map[string]plan.NodeKind{dest: plan.KindDir, dest + "/a": plan.KindDir, dest + "/a/b": plan.KindLink}, changes,
			[]string{"a/b/new.lua"}},
		{"file entry over a destination link is replaced, not followed", plan.KindFile, map[string]plan.NodeKind{dest: plan.KindLink}, changes, nil},
		{"deletes are not write targets", plan.KindDir, map[string]plan.NodeKind{dest: plan.KindDir, dest + "/a": plan.KindLink},
			[]plan.Change{{Path: "a/gone.lua", Type: plan.Delete}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfgsync.PreflightBlocked(dest, tt.srcKind, tt.changes, kinds(tt.kinds))
			if !slices.Equal(got, tt.want) {
				t.Errorf("PreflightBlocked() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPreflightBlockedFailsClosedWhenPathCannotBeInspected(t *testing.T) {
	kindOf := func(string) (plan.NodeKind, error) { return plan.KindNone, errors.New("permission denied") }
	got := cfgsync.PreflightBlocked("/d", plan.KindDir, []plan.Change{{Path: "a/x", Type: plan.Create}}, kindOf)
	if !slices.Equal(got, []string{"a/x"}) {
		t.Errorf("PreflightBlocked() = %q, want [a/x]", got)
	}
}
