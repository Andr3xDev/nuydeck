package domain

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
)

var (
	ErrUnsupportedVersion = errors.New("unsupported manifest version")
	ErrUnknownRoot        = errors.New("unknown root")
	ErrInvalidPath        = errors.New("invalid entry path")
	ErrDuplicateEntry     = errors.New("duplicate entry")
	ErrNestedEntry        = errors.New("nested entry")
	ErrInsideBackups      = errors.New("entry inside backups directory")
	ErrSecretsPath        = errors.New("entry resolves to secrets")
)

const (
	supportedManifestVersion = 1
	backupsDirName           = ".nuydeck-backups"
	secretsDirName           = ".secrets"
	globChars                = "*?["
)

var knownRoots = []Root{RootConfig, RootHome, RootFonts, RootThemes}

// EntryError ties a validation failure to the index of the offending entry;
// Prior is the index of the entry it collides with, or -1.
type EntryError struct {
	Index int
	Prior int
	Err   error
}

func (e *EntryError) Error() string { return e.Err.Error() }
func (e *EntryError) Unwrap() error { return e.Err }

// Validate checks the manifest; Roots must already be absolute (home is used for the ~/.secrets and ~/.nuydeck-backups rules).
func (m Manifest) Validate(home string) error {
	if m.Version != supportedManifestVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedVersion, m.Version)
	}
	for root := range m.Roots {
		if !slices.Contains(knownRoots, root) {
			return fmt.Errorf("%w: %q in [roots]", ErrUnknownRoot, root)
		}
	}
	backupsDir := path.Join(home, backupsDirName)
	secretsDir := path.Join(home, secretsDirName)
	for i, e := range m.Entries {
		entryErr := func(prior int, err error) error { return &EntryError{Index: i, Prior: prior, Err: err} }
		if _, ok := m.Roots[e.Root]; !ok || !slices.Contains(knownRoots, e.Root) {
			return entryErr(-1, fmt.Errorf("%w: %q", ErrUnknownRoot, e.Root))
		}
		if err := validateEntryPath(e.Path); err != nil {
			return entryErr(-1, err)
		}
		resolved := path.Join(m.Roots[e.Root], e.Path)
		if isWithin(resolved, backupsDir) {
			return entryErr(-1, fmt.Errorf("%w: %s/%s", ErrInsideBackups, e.Root, e.Path))
		}
		if isWithin(resolved, secretsDir) {
			return entryErr(-1, fmt.Errorf("%w: %s/%s", ErrSecretsPath, e.Root, e.Path))
		}
		for j, other := range m.Entries[:i] {
			if other.Root != e.Root {
				continue
			}
			switch {
			case other.Path == e.Path:
				return entryErr(j, fmt.Errorf("%w: %s/%s", ErrDuplicateEntry, e.Root, e.Path))
			case isWithin(e.Path, other.Path), isWithin(other.Path, e.Path):
				return entryErr(j, fmt.Errorf("%w: %s/%s and %s/%s", ErrNestedEntry, e.Root, e.Path, other.Root, other.Path))
			}
		}
	}
	return nil
}

func validateEntryPath(p string) error {
	switch {
	case p == "":
		return fmt.Errorf("%w: path is empty", ErrInvalidPath)
	case path.IsAbs(p), p == "..", strings.HasPrefix(p, "../"), path.Clean(p) != p, p == ".",
		strings.ContainsAny(p, globChars):
		return fmt.Errorf("%w: %q", ErrInvalidPath, p)
	}
	return nil
}

// isWithin reports whether p equals dir or lies below it.
func isWithin(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}

// FiltersFor translates the excludes of the entry's root into rsync rules anchored at the entry;
// the bool reports that the whole entry is excluded.
func FiltersFor(entry Entry, excludes []Exclude) (Filter, bool) {
	var filter Filter
	for _, x := range excludes {
		if x.Root != entry.Root {
			continue
		}
		if isWithin(entry.Path, strings.TrimSuffix(x.Pattern, "/")) {
			return Filter{}, true
		}
		if rest, ok := strings.CutPrefix(x.Pattern, entry.Path+"/"); ok {
			filter.Excludes = append(filter.Excludes, "/"+rest)
		}
	}
	return filter, false
}

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
		if e.Root == root && isWithin(resolved, e.Path) {
			return NoLinkWarning
		}
	}
	return WarnLeavesManagedSet
}

// PreflightBlocked returns the relative paths of created/updated changes that must not be written
// because destDir or one of their parent directories is a symlink (or cannot be inspected).
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
