package manifest_test

import (
	"errors"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func pkg(name string, source manifest.Source, profiles ...string) manifest.Package {
	return manifest.Package{Name: name, Source: source, Profiles: profiles}
}

func TestCatalogValidatePackages(t *testing.T) {
	tests := []struct {
		name      string
		pkgs      []manifest.Package
		want      error
		wantIndex int
		wantPrior int
	}{
		{"valid with optional fields absent", []manifest.Package{
			pkg("hyprland", manifest.SourcePacman, "minimal", "full"),
			pkg("zen-browser-bin", manifest.SourceAUR, "full"),
		}, nil, 0, 0},
		{"unknown source", []manifest.Package{pkg("x", "flatpak", "full")}, manifest.ErrUnknownSource, 0, -1},
		{"empty source", []manifest.Package{pkg("x", "", "full")}, manifest.ErrUnknownSource, 0, -1},
		{"unknown profile", []manifest.Package{pkg("x", manifest.SourcePacman, "minimal", "huge")}, manifest.ErrUnknownProfile, 0, -1},
		{"empty name", []manifest.Package{pkg("", manifest.SourcePacman, "full")}, manifest.ErrEmptyPackageName, 0, -1},
		{"name starting with dash", []manifest.Package{pkg("-Syu", manifest.SourcePacman, "full")}, manifest.ErrInvalidPackageName, 0, -1},
		{"name with whitespace", []manifest.Package{pkg("a b", manifest.SourcePacman, "full")}, manifest.ErrInvalidPackageName, 0, -1},
		{"duplicate name names both entries", []manifest.Package{
			pkg("a", manifest.SourcePacman, "full"),
			pkg("b", manifest.SourcePacman, "full"),
			pkg("a", manifest.SourceAUR, "full"),
		}, manifest.ErrDuplicatePackage, 2, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := manifest.ValidatePackages(tc.pkgs)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var entryErr *manifest.EntryError
			if !errors.Is(err, tc.want) || !errors.As(err, &entryErr) {
				t.Fatalf("err = %v, want EntryError wrapping %v", err, tc.want)
			}
			if entryErr.Index != tc.wantIndex || entryErr.Prior != tc.wantPrior {
				t.Errorf("Index/Prior = %d/%d, want %d/%d", entryErr.Index, entryErr.Prior, tc.wantIndex, tc.wantPrior)
			}
		})
	}
}

func TestCatalogValidateDrivers(t *testing.T) {
	tests := []struct {
		name    string
		drivers []manifest.Driver
		want    error
	}{
		{"valid, uppercase hex accepted", []manifest.Driver{
			{ID: "nvidia", VendorID: "10DE"}, {ID: "amd", VendorID: "1002", Packages: []string{"mesa"}},
		}, nil},
		{"duplicate id", []manifest.Driver{
			{ID: "amd", VendorID: "1002"}, {ID: "amd", VendorID: "1003"},
		}, manifest.ErrDuplicateDriver},
		{"empty driver id", []manifest.Driver{{ID: "", VendorID: "10de"}}, manifest.ErrInvalidDriverID},
		{"driver id uppercase", []manifest.Driver{{ID: "NVIDIA", VendorID: "10de"}}, manifest.ErrInvalidDriverID},
		{"driver id with separator", []manifest.Driver{{ID: "a_b", VendorID: "10de"}}, manifest.ErrInvalidDriverID},
		{"vendor id too short", []manifest.Driver{{ID: "x", VendorID: "10d"}}, manifest.ErrInvalidVendorID},
		{"vendor id too long", []manifest.Driver{{ID: "x", VendorID: "10de0"}}, manifest.ErrInvalidVendorID},
		{"vendor id not hex", []manifest.Driver{{ID: "x", VendorID: "10zz"}}, manifest.ErrInvalidVendorID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := manifest.ValidateDrivers(tc.drivers)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
