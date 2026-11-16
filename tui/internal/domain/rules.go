package domain

import (
	"path"
	"strings"
)

// ClassifyLink inspects a link target; linkDir is the link's directory relative to root and entries are the managed entries.
func ClassifyLink(root Root, linkDir, target string, entries []Entry) LinkWarning {
	if path.IsAbs(target) {
		return WarnAbsoluteLink
	}
	resolved := path.Join(linkDir, target)
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return WarnLeavesManagedSet
	}
	for _, e := range entries {
		if e.Root == root && IsWithin(resolved, e.Path) {
			return NoLinkWarning
		}
	}
	return WarnLeavesManagedSet
}

// PreflightBlocked returns the relative paths of created/updated changes that must not be written
// because destDir or one of their parent directories is a symlink (or cannot be inspected).
// kindOf must return KindNone, not an error, for missing paths (ENOENT): any error blocks.
func PreflightBlocked(destDir string, srcKind NodeKind, changes []Change, kindOf func(path string) (NodeKind, error)) []string {
	if srcKind != KindDir {
		return nil
	}
	var blocked []string
	for _, c := range changes {
		if c.Type != Create && c.Type != Update {
			continue
		}
		if hasLinkedAncestor(destDir, c.Path, kindOf) {
			blocked = append(blocked, c.Path)
		}
	}
	return blocked
}

func hasLinkedAncestor(destDir, rel string, kindOf func(string) (NodeKind, error)) bool {
	dir := destDir
	for _, segment := range append([]string{""}, strings.Split(path.Dir(rel), "/")...) {
		if segment == "." {
			break
		}
		dir = path.Join(dir, segment)
		if kind, err := kindOf(dir); err != nil || kind == KindLink {
			return true
		}
	}
	return false
}
