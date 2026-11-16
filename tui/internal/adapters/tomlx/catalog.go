package tomlx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/fsutil"
)

const (
	packageTable = "package"
	driverTable  = "driver"
	secretTable  = "secret"
)

type packageDoc struct {
	Version int          `toml:"version"`
	Package []packageRow `toml:"package"`
}

type packageRow struct {
	Name     string   `toml:"name"`
	Source   string   `toml:"source"`
	Profiles []string `toml:"profiles"`
	Group    string   `toml:"group"`
	Service  string   `toml:"service"`
}

type hardwareDoc struct {
	Version int         `toml:"version"`
	Driver  []driverRow `toml:"driver"`
}

type driverRow struct {
	ID       string   `toml:"id"`
	VendorID string   `toml:"vendor_id"`
	Packages []string `toml:"packages"`
	Services []string `toml:"services"`
}

type secretsDoc struct {
	Version int         `toml:"version"`
	Secret  []secretRow `toml:"secret"`
}

// secretRow has no value field on purpose: strict decoding rejects one.
type secretRow struct {
	Name string `toml:"name"`
}

// LoadPackages reads and validates the package catalog.
func LoadPackages(path string) ([]domain.Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parsePackages(filepath.Base(path), data)
}

func parsePackages(name string, data []byte) ([]domain.Package, error) {
	var doc packageDoc
	if err := decodeStrict(name, data, &doc); err != nil {
		return nil, err
	}
	pkgs := make([]domain.Package, len(doc.Package))
	for i, p := range doc.Package {
		pkgs[i] = domain.Package{Name: p.Name, Source: domain.Source(p.Source), Profiles: p.Profiles, Group: p.Group, Service: p.Service}
	}
	if err := domain.ValidatePackages(pkgs); err != nil {
		return nil, entryFailure(name, data, packageTable, err)
	}
	return pkgs, nil
}

// LoadHardware reads and validates the GPU driver catalog.
func LoadHardware(path string) ([]domain.Driver, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	var doc hardwareDoc
	if err := decodeStrict(name, data, &doc); err != nil {
		return nil, err
	}
	drivers := make([]domain.Driver, len(doc.Driver))
	for i, d := range doc.Driver {
		drivers[i] = domain.Driver{ID: d.ID, VendorID: d.VendorID, Packages: d.Packages, Services: d.Services}
	}
	if err := domain.ValidateDrivers(drivers); err != nil {
		return nil, entryFailure(name, data, driverTable, err)
	}
	return drivers, nil
}

// LoadSecrets returns the secret names; an absent file means no names.
func LoadSecrets(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc secretsDoc
	if err := decodeStrict(filepath.Base(path), data, &doc); err != nil {
		return nil, err
	}
	var names []string
	for _, s := range doc.Secret {
		names = append(names, s.Name)
	}
	if err := domain.ValidateSecretNames(names); err != nil {
		return nil, entryFailure(filepath.Base(path), data, secretTable, err)
	}
	return names, nil
}

// AppendPackage adds one [[package]] block after the existing text, keeping comments and order.
// The candidate is validated before it replaces the file.
func AppendPackage(path string, entry domain.Package) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	candidate := string(existing)
	if candidate != "" && !strings.HasSuffix(candidate, "\n") {
		candidate += "\n"
	}
	candidate += renderPackage(entry)
	if _, err := parsePackages(filepath.Base(path), []byte(candidate)); err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, []byte(candidate), info.Mode().Perm())
}

func renderPackage(p domain.Package) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n[[package]]\nname = %s\nsource = %s\n", strconv.Quote(p.Name), strconv.Quote(string(p.Source)))
	if len(p.Profiles) > 0 {
		quoted := make([]string, len(p.Profiles))
		for i, profile := range p.Profiles {
			quoted[i] = strconv.Quote(profile)
		}
		fmt.Fprintf(&b, "profiles = [%s]\n", strings.Join(quoted, ", "))
	}
	if p.Group != "" {
		fmt.Fprintf(&b, "group = %s\n", strconv.Quote(p.Group))
	}
	if p.Service != "" {
		fmt.Fprintf(&b, "service = %s\n", strconv.Quote(p.Service))
	}
	return b.String()
}
