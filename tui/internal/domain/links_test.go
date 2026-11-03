package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func TestClassifyLink(t *testing.T) {
	entries := []domain.Entry{
		entry("config", "starship"), entry("config", "starship.toml"),
		entry("config", "hypr"), entry("config", "nvim"), entry("config", "dunst"),
	}
	tests := []struct {
		name    string
		linkDir string
		target  string
		want    domain.LinkWarning
	}{
		{"absolute target", "", "/usr/share/starship.toml", domain.WarnAbsoluteLink},
		{"absolute target pointing into the managed set", "hypr", "/home/test/.config/hypr/x", domain.WarnAbsoluteLink},
		{"variant link stays inside a sibling entry", "", "starship/nuytek-nadir/starship.toml", domain.NoLinkWarning},
		{"sibling file inside own entry", "dunst", "dunstrc-nadir", domain.NoLinkWarning},
		{"climbs into another managed entry", "hypr", "../nvim/init.lua", domain.NoLinkWarning},
		{"points at an entry itself", "hypr", "../nvim", domain.NoLinkWarning},
		{"resolves to an unmanaged sibling", "hypr", "../btop/btop.conf", domain.WarnLeavesManagedSet},
		{"resolves to unmanaged file at root", "hypr/theme", "../../foo", domain.WarnLeavesManagedSet},
		{"climbs above the root", "hypr/theme", "../../../.ssh/config", domain.WarnLeavesManagedSet},
		{"unclean relative target is cleaned first", "hypr", "./theme/../colors.conf", domain.NoLinkWarning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.ClassifyLink("config", tt.linkDir, tt.target, entries); got != tt.want {
				t.Errorf("ClassifyLink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyLinkIgnoresEntriesOfOtherRoots(t *testing.T) {
	entries := []domain.Entry{entry("home", "nvim")}
	if got := domain.ClassifyLink("config", "hypr", "../nvim/init.lua", entries); got != domain.WarnLeavesManagedSet {
		t.Errorf("ClassifyLink() = %v, want WarnLeavesManagedSet", got)
	}
}

func TestPreflightBlocked(t *testing.T) {
	const dest = "/home/test/.config/hypr"
	kinds := func(m map[string]domain.NodeKind) func(string) (domain.NodeKind, error) {
		return func(p string) (domain.NodeKind, error) { return m[p], nil } // missing path = KindNone
	}
	changes := []domain.Change{
		{Path: "a/b/new.lua", Type: domain.Create},
		{Path: "a/old.lua", Type: domain.Update},
		{Path: "c/other.lua", Type: domain.Update},
	}
	tests := []struct {
		name    string
		srcKind domain.NodeKind
		kinds   map[string]domain.NodeKind
		changes []domain.Change
		want    []string
	}{
		{"real dirs are fine", domain.KindDir, map[string]domain.NodeKind{dest: domain.KindDir, dest + "/a": domain.KindDir, dest + "/c": domain.KindDir}, changes, nil},
		{"missing parents are fine", domain.KindDir, map[string]domain.NodeKind{dest: domain.KindDir}, changes, nil},
		{"destination root symlinked blocks everything", domain.KindDir, map[string]domain.NodeKind{dest: domain.KindLink}, changes,
			[]string{"a/b/new.lua", "a/old.lua", "c/other.lua"}},
		{"symlinked parent blocks only items below it", domain.KindDir, map[string]domain.NodeKind{dest: domain.KindDir, dest + "/a": domain.KindLink}, changes,
			[]string{"a/b/new.lua", "a/old.lua"}},
		{"deep symlinked parent blocks only its subtree", domain.KindDir, map[string]domain.NodeKind{dest: domain.KindDir, dest + "/a": domain.KindDir, dest + "/a/b": domain.KindLink}, changes,
			[]string{"a/b/new.lua"}},
		{"file entry over a destination link is replaced, not followed", domain.KindFile, map[string]domain.NodeKind{dest: domain.KindLink}, changes, nil},
		{"deletes are not write targets", domain.KindDir, map[string]domain.NodeKind{dest: domain.KindDir, dest + "/a": domain.KindLink},
			[]domain.Change{{Path: "a/gone.lua", Type: domain.Delete}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.PreflightBlocked(dest, tt.srcKind, tt.changes, kinds(tt.kinds))
			if !slices.Equal(got, tt.want) {
				t.Errorf("PreflightBlocked() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPreflightBlockedFailsClosedWhenPathCannotBeInspected(t *testing.T) {
	kindOf := func(string) (domain.NodeKind, error) { return domain.KindNone, errors.New("permission denied") }
	got := domain.PreflightBlocked("/d", domain.KindDir, []domain.Change{{Path: "a/x", Type: domain.Create}}, kindOf)
	if !slices.Equal(got, []string{"a/x"}) {
		t.Errorf("PreflightBlocked() = %q, want [a/x]", got)
	}
}
