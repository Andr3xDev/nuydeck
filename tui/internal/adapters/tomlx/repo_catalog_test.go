package tomlx_test

import (
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/tomlx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func TestRepoCatalogKeepsContentRules(t *testing.T) {
	pkgs, err := tomlx.LoadPackages("../../../../manifest/packages.toml")
	if err != nil {
		t.Fatalf("LoadPackages() error = %v", err)
	}
	byName := map[string]domain.Package{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}

	for _, banned := range []string{
		"sunshine", "xf86-video-nouveau", "vulkan-nouveau", "polkit-kde-agent", "wofi", "rofi", "firefox", "swww",
	} {
		if _, ok := byName[banned]; ok {
			t.Errorf("catalog lists removed package %q", banned)
		}
	}
	if p, ok := byName["xsettingsd"]; !ok || !slices.Contains(p.Profiles, domain.ProfileMinimal) {
		t.Errorf("xsettingsd = %+v, want present in minimal", p)
	}
	if p, ok := byName["zen-browser-bin"]; !ok || p.Source != domain.SourceAUR {
		t.Errorf("zen-browser-bin = %+v, want present with source aur", p)
	}
	if p, ok := byName["awww"]; !ok || p.Source != domain.SourcePacman {
		t.Errorf("awww = %+v, want present with source pacman", p)
	}
}

func TestRepoHardwareCatalogHasThreeDrivers(t *testing.T) {
	drivers, err := tomlx.LoadHardware("../../../../manifest/hardware.toml")
	if err != nil {
		t.Fatalf("LoadHardware() error = %v", err)
	}
	byID := map[string]domain.Driver{}
	for _, d := range drivers {
		byID[d.ID] = d
	}
	if len(drivers) != 3 {
		t.Errorf("drivers = %d, want 3", len(drivers))
	}
	for id, vendor := range map[string]string{"intel": "8086", "nvidia": "10de", "amd": "1002"} {
		if byID[id].VendorID != vendor {
			t.Errorf("driver %q vendor = %q, want %q", id, byID[id].VendorID, vendor)
		}
	}
	if len(byID["nvidia"].Services) != 3 {
		t.Errorf("nvidia services = %v, want 3 units", byID["nvidia"].Services)
	}
	if len(byID["intel"].Services) != 0 {
		t.Errorf("intel services = %v, want none", byID["intel"].Services)
	}
}

func TestRepoSecretsManifestIsEmpty(t *testing.T) {
	names, err := tomlx.LoadSecrets("../../../../manifest/secrets.toml")
	if err != nil {
		t.Fatalf("LoadSecrets() error = %v", err)
	}
	if len(names) != 0 {
		t.Errorf("names = %v, want none", names)
	}
}
