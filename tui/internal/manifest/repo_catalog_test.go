package manifest_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func TestRepoCatalogKeepsContentRules(t *testing.T) {
	pkgs, err := manifest.LoadPackages("../../../manifest/packages.toml")
	if err != nil {
		t.Fatalf("LoadPackages() error = %v", err)
	}
	byName := map[string]manifest.Package{}
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
	if p, ok := byName["xsettingsd"]; !ok || !slices.Contains(p.Profiles, manifest.ProfileMinimal) {
		t.Errorf("xsettingsd = %+v, want present in minimal", p)
	}
	if p, ok := byName["zen-browser-bin"]; !ok || p.Source != manifest.SourceAUR {
		t.Errorf("zen-browser-bin = %+v, want present with source aur", p)
	}
	if p, ok := byName["awww"]; !ok || p.Source != manifest.SourcePacman {
		t.Errorf("awww = %+v, want present with source pacman", p)
	}
	if p, ok := byName["ttf-ms-fonts"]; !ok || p.Source != manifest.SourceAUR || !slices.Equal(p.Profiles, []string{manifest.ProfileFull}) {
		t.Errorf("ttf-ms-fonts = %+v, want aur and full only", p)
	}
	for _, name := range []string{"sassc", "git"} {
		if p, ok := byName[name]; !ok || !slices.Contains(p.Profiles, manifest.ProfileMinimal) {
			t.Errorf("%s = %+v, want present in minimal", name, p)
		}
	}
}

func TestRepoPathsManifestHygiene(t *testing.T) {
	m, err := manifest.LoadManifest("../../../manifest/paths.toml", "/home/test")
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	have := map[string]bool{}
	for _, e := range m.Entries {
		have[string(e.Root)+"/"+e.Path] = true
		if e.Root == manifest.RootThemes || strings.Contains(e.Path, "gtk-4.0") || strings.Contains(e.Path, "spicetify") {
			t.Errorf("entry %s/%s must not be managed", e.Root, e.Path)
		}
	}
	if !have["config/xsettingsd"] {
		t.Error("entry config/xsettingsd missing")
	}
	if m.Roots[manifest.RootThemes] == "" {
		t.Error("themes root must stay defined")
	}
}

func TestRepoHardwareCatalogHasThreeDrivers(t *testing.T) {
	drivers, err := manifest.LoadHardware("../../../manifest/hardware.toml")
	if err != nil {
		t.Fatalf("LoadHardware() error = %v", err)
	}
	byID := map[string]manifest.Driver{}
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
	names, err := manifest.LoadSecrets("../../../manifest/secrets.toml")
	if err != nil {
		t.Fatalf("LoadSecrets() error = %v", err)
	}
	if len(names) != 0 {
		t.Errorf("names = %v, want none", names)
	}
}
