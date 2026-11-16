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
	ErrInvalidRoot        = errors.New("invalid root destination")
	ErrInvalidExclude     = errors.New("invalid exclude")
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
	rootNames := make([]Root, 0, len(m.Roots))
	for root := range m.Roots {
		rootNames = append(rootNames, root)
	}
	slices.Sort(rootNames)
	for _, root := range rootNames {
		if !slices.Contains(knownRoots, root) {
			return fmt.Errorf("%w: %q in [roots]", ErrUnknownRoot, root)
		}
		if dest := m.Roots[root]; !path.IsAbs(dest) {
			return fmt.Errorf("%w: %s = %q", ErrInvalidRoot, root, dest)
		}
	}
	for _, x := range m.Excludes {
		if err := validateExclude(x, m.Roots); err != nil {
			return err
		}
	}
	backupsDir := path.Join(home, backupsDirName)
	secretsDir := path.Join(home, secretsDirName)
	for i, e := range m.Entries {
		entryErr := func(prior int, err error) error { return &EntryError{Index: i, Prior: prior, Err: err} }
		if _, ok := m.Roots[e.Root]; !ok {
			return entryErr(-1, fmt.Errorf("%w: %q", ErrUnknownRoot, e.Root))
		}
		if err := validateEntryPath(e.Path); err != nil {
			return entryErr(-1, err)
		}
		resolved := path.Join(m.Roots[e.Root], e.Path)
		if IsWithin(resolved, backupsDir) || IsWithin(backupsDir, resolved) {
			return entryErr(-1, fmt.Errorf("%w: %s/%s", ErrInsideBackups, e.Root, e.Path))
		}
		if IsWithin(resolved, secretsDir) || IsWithin(secretsDir, resolved) {
			return entryErr(-1, fmt.Errorf("%w: %s/%s", ErrSecretsPath, e.Root, e.Path))
		}
		for j, other := range m.Entries[:i] {
			if other.Root != e.Root {
				continue
			}
			switch {
			case other.Path == e.Path:
				return entryErr(j, fmt.Errorf("%w: %s/%s", ErrDuplicateEntry, e.Root, e.Path))
			case IsWithin(e.Path, other.Path), IsWithin(other.Path, e.Path):
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

func validateExclude(x Exclude, roots map[Root]string) error {
	if _, ok := roots[x.Root]; !ok {
		return fmt.Errorf("%w: unknown root %q", ErrInvalidExclude, x.Root)
	}
	if x.Pattern == "" || path.IsAbs(x.Pattern) || slices.Contains(strings.Split(x.Pattern, "/"), "..") {
		return fmt.Errorf("%w: %s/%q", ErrInvalidExclude, x.Root, x.Pattern)
	}
	return nil
}

// IsWithin reports whether p equals dir or lies below it.
func IsWithin(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}
