package manifest

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var (
	ErrUnknownSource    = errors.New("unknown package source")
	ErrUnknownProfile   = errors.New("unknown profile")
	ErrEmptyPackageName = errors.New("empty package name")
	ErrDuplicatePackage = errors.New("duplicate package")
	ErrDuplicateDriver  = errors.New("duplicate driver")
	ErrInvalidVendorID  = errors.New("invalid vendor id")
	ErrInvalidDriverID  = errors.New("invalid driver id")

	ErrInvalidPackageName = errors.New("invalid package name")
	ErrInvalidSecretName  = errors.New("invalid secret name")
	ErrDuplicateSecret    = errors.New("duplicate secret")
)

// ParuPrerequisites are the packages paru needs before it can be built from the AUR.
var ParuPrerequisites = []string{"base-devel", "git"}

type Source string

const (
	SourcePacman Source = "pacman"
	SourceAUR    Source = "aur"
)

const vendorIDLength = 4

// Driver IDs end up in the gpu_env_<ids>.lua file name.
var driverIDPattern = regexp.MustCompile(`^[a-z0-9]+$`)

var secretNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const (
	ProfileMinimal = "minimal"
	ProfileFull    = "full"
)

var knownProfiles = []string{ProfileMinimal, ProfileFull}

type Package struct {
	Name     string
	Source   Source
	Profiles []string
	Group    string
	Service  string
}

type Driver struct {
	ID       string
	VendorID string
	Packages []string
	Services []string
}

func ValidatePackages(pkgs []Package) error {
	seen := make(map[string]int, len(pkgs))
	for i, p := range pkgs {
		if p.Name == "" {
			return entryError(i, -1, ErrEmptyPackageName)
		}
		if strings.HasPrefix(p.Name, "-") || strings.ContainsFunc(p.Name, unicode.IsSpace) {
			return entryError(i, -1, fmt.Errorf("%w: %q", ErrInvalidPackageName, p.Name))
		}
		if p.Source != SourcePacman && p.Source != SourceAUR {
			return entryError(i, -1, fmt.Errorf("%w: %q for %s", ErrUnknownSource, p.Source, p.Name))
		}
		for _, profile := range p.Profiles {
			if !slices.Contains(knownProfiles, profile) {
				return entryError(i, -1, fmt.Errorf("%w: %q for %s", ErrUnknownProfile, profile, p.Name))
			}
		}
		if prior, dup := seen[p.Name]; dup {
			return entryError(i, prior, fmt.Errorf("%w: %s", ErrDuplicatePackage, p.Name))
		}
		seen[p.Name] = i
	}
	return nil
}

func ValidateDrivers(drivers []Driver) error {
	seen := make(map[string]int, len(drivers))
	for i, d := range drivers {
		if !driverIDPattern.MatchString(d.ID) {
			return entryError(i, -1, fmt.Errorf("%w: %q", ErrInvalidDriverID, d.ID))
		}
		if !isHexID(d.VendorID) {
			return entryError(i, -1, fmt.Errorf("%w: %q for %s", ErrInvalidVendorID, d.VendorID, d.ID))
		}
		if prior, dup := seen[d.ID]; dup {
			return entryError(i, prior, fmt.Errorf("%w: %s", ErrDuplicateDriver, d.ID))
		}
		seen[d.ID] = i
	}
	return nil
}

// ValidateSecretNames accepts shell-safe variable names only: they are written unquoted into `export NAME=...`.
func ValidateSecretNames(names []string) error {
	seen := make(map[string]int, len(names))
	for i, name := range names {
		if !secretNamePattern.MatchString(name) {
			return entryError(i, -1, fmt.Errorf("%w: %q", ErrInvalidSecretName, name))
		}
		if prior, dup := seen[name]; dup {
			return entryError(i, prior, fmt.Errorf("%w: %s", ErrDuplicateSecret, name))
		}
		seen[name] = i
	}
	return nil
}

func entryError(index, prior int, err error) error {
	return &EntryError{Index: index, Prior: prior, Err: err}
}

func isHexID(s string) bool {
	if len(s) != vendorIDLength {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}
