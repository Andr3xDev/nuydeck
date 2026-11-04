package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func pkg(name string, source domain.Source, profiles ...string) domain.Package {
	return domain.Package{Name: name, Source: source, Profiles: profiles}
}

func TestCatalogValidatePackages(t *testing.T) {
	tests := []struct {
		name      string
		pkgs      []domain.Package
		want      error
		wantIndex int
		wantPrior int
	}{
		{"valid with optional fields absent", []domain.Package{
			pkg("hyprland", domain.SourcePacman, "minimal", "full"),
			pkg("zen-browser-bin", domain.SourceAUR, "full"),
		}, nil, 0, 0},
		{"unknown source", []domain.Package{pkg("x", "flatpak", "full")}, domain.ErrUnknownSource, 0, -1},
		{"empty source", []domain.Package{pkg("x", "", "full")}, domain.ErrUnknownSource, 0, -1},
		{"unknown profile", []domain.Package{pkg("x", domain.SourcePacman, "minimal", "huge")}, domain.ErrUnknownProfile, 0, -1},
		{"empty name", []domain.Package{pkg("", domain.SourcePacman, "full")}, domain.ErrEmptyPackageName, 0, -1},
		{"name starting with dash", []domain.Package{pkg("-Syu", domain.SourcePacman, "full")}, domain.ErrInvalidPackageName, 0, -1},
		{"name with whitespace", []domain.Package{pkg("a b", domain.SourcePacman, "full")}, domain.ErrInvalidPackageName, 0, -1},
		{"duplicate name names both entries", []domain.Package{
			pkg("a", domain.SourcePacman, "full"),
			pkg("b", domain.SourcePacman, "full"),
			pkg("a", domain.SourceAUR, "full"),
		}, domain.ErrDuplicatePackage, 2, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidatePackages(tc.pkgs)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var entryErr *domain.EntryError
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
		drivers []domain.Driver
		want    error
	}{
		{"valid, uppercase hex accepted", []domain.Driver{
			{ID: "nvidia", VendorID: "10DE"}, {ID: "amd", VendorID: "1002", Packages: []string{"mesa"}},
		}, nil},
		{"duplicate id", []domain.Driver{
			{ID: "amd", VendorID: "1002"}, {ID: "amd", VendorID: "1003"},
		}, domain.ErrDuplicateDriver},
		{"empty driver id", []domain.Driver{{ID: "", VendorID: "10de"}}, domain.ErrInvalidDriverID},
		{"driver id uppercase", []domain.Driver{{ID: "NVIDIA", VendorID: "10de"}}, domain.ErrInvalidDriverID},
		{"driver id with separator", []domain.Driver{{ID: "a_b", VendorID: "10de"}}, domain.ErrInvalidDriverID},
		{"vendor id too short", []domain.Driver{{ID: "x", VendorID: "10d"}}, domain.ErrInvalidVendorID},
		{"vendor id too long", []domain.Driver{{ID: "x", VendorID: "10de0"}}, domain.ErrInvalidVendorID},
		{"vendor id not hex", []domain.Driver{{ID: "x", VendorID: "10zz"}}, domain.ErrInvalidVendorID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateDrivers(tc.drivers)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCatalogUnregistered(t *testing.T) {
	catalog := []domain.Package{pkg("hyprland", domain.SourcePacman, "full")}
	drivers := []domain.Driver{{ID: "nvidia", VendorID: "10de", Packages: []string{"nvidia-utils"}}}
	prerequisites := []string{"base-devel", "git"}

	tests := []struct {
		name     string
		explicit []string
		ignored  []string
		want     []string
	}{
		{"unregistered listed", []string{"hyprland", "htop"}, nil, []string{"htop"}},
		{"catalog, driver and prerequisite packages not listed",
			[]string{"hyprland", "nvidia-utils", "base-devel", "git"}, nil, []string{}},
		{"ignored not listed", []string{"htop", "tmux"}, []string{"htop"}, []string{"tmux"}},
		{"nothing unregistered", []string{"hyprland"}, nil, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.Unregistered(tc.explicit, catalog, drivers, prerequisites, tc.ignored)
			if got == nil {
				t.Fatal("Unregistered() = nil, want a non-nil slice")
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Unregistered() = %v, want %v", got, tc.want)
			}
		})
	}
}
