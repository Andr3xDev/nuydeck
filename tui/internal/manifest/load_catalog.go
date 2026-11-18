package manifest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
func LoadPackages(path string) ([]Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parsePackages(filepath.Base(path), data)
}

func parsePackages(name string, data []byte) ([]Package, error) {
	var doc packageDoc
	if err := DecodeStrict(name, data, &doc); err != nil {
		return nil, err
	}
	pkgs := make([]Package, len(doc.Package))
	for i, p := range doc.Package {
		pkgs[i] = Package{Name: p.Name, Source: Source(p.Source), Profiles: p.Profiles, Group: p.Group, Service: p.Service}
	}
	if err := ValidatePackages(pkgs); err != nil {
		return nil, EntryFailure(name, data, packageTable, err)
	}
	return pkgs, nil
}

// LoadHardware reads and validates the GPU driver catalog.
func LoadHardware(path string) ([]Driver, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	var doc hardwareDoc
	if err := DecodeStrict(name, data, &doc); err != nil {
		return nil, err
	}
	drivers := make([]Driver, len(doc.Driver))
	for i, d := range doc.Driver {
		drivers[i] = Driver{ID: d.ID, VendorID: d.VendorID, Packages: d.Packages, Services: d.Services}
	}
	if err := ValidateDrivers(drivers); err != nil {
		return nil, EntryFailure(name, data, driverTable, err)
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
	if err := DecodeStrict(filepath.Base(path), data, &doc); err != nil {
		return nil, err
	}
	var names []string
	for _, s := range doc.Secret {
		names = append(names, s.Name)
	}
	if err := ValidateSecretNames(names); err != nil {
		return nil, EntryFailure(filepath.Base(path), data, secretTable, err)
	}
	return names, nil
}
