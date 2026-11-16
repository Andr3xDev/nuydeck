package domain

import (
	"path"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// ClassifyLink inspects a link target; linkDir is the link's directory relative to root and entries are the managed entries.
func ClassifyLink(root manifest.Root, linkDir, target string, entries []manifest.Entry) plan.LinkWarning {
	if path.IsAbs(target) {
		return plan.WarnAbsoluteLink
	}
	resolved := path.Join(linkDir, target)
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return plan.WarnLeavesManagedSet
	}
	for _, e := range entries {
		if e.Root == root && manifest.IsWithin(resolved, e.Path) {
			return plan.NoLinkWarning
		}
	}
	return plan.WarnLeavesManagedSet
}

// PreflightBlocked returns the relative paths of created/updated changes that must not be written
// because destDir or one of their parent directories is a symlink (or cannot be inspected).
// kindOf must return KindNone, not an error, for missing paths (ENOENT): any error blocks.
func PreflightBlocked(destDir string, srcKind plan.NodeKind, changes []plan.Change, kindOf func(path string) (plan.NodeKind, error)) []string {
	if srcKind != plan.KindDir {
		return nil
	}
	var blocked []string
	for _, c := range changes {
		if c.Type != plan.Create && c.Type != plan.Update {
			continue
		}
		if hasLinkedAncestor(destDir, c.Path, kindOf) {
			blocked = append(blocked, c.Path)
		}
	}
	return blocked
}

func hasLinkedAncestor(destDir, rel string, kindOf func(string) (plan.NodeKind, error)) bool {
	dir := destDir
	for _, segment := range append([]string{""}, strings.Split(path.Dir(rel), "/")...) {
		if segment == "." {
			break
		}
		dir = path.Join(dir, segment)
		if kind, err := kindOf(dir); err != nil || kind == plan.KindLink {
			return true
		}
	}
	return false
}
