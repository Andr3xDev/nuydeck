package packages

import (
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func catalogPkg(name, group string, source manifest.Source) manifest.Package {
	return manifest.Package{Name: name, Group: group, Source: source, Profiles: []string{"minimal"}}
}

func TestRows_ClassifiesCatalogPackagesAndDrift(t *testing.T) {
	catalog := []manifest.Package{
		catalogPkg("hyprland", "desktop", manifest.SourcePacman),
		catalogPkg("waybar", "desktop", manifest.SourcePacman),
		catalogPkg("paru-bin", "tools", manifest.SourceAUR),
	}
	installed := map[string]bool{"hyprland": true, "htop": true}
	explicit := []string{"hyprland", "htop"}

	got := Rows(installed, explicit, catalog, nil, nil, nil)

	want := []Row{
		{Name: "hyprland", Group: "desktop", Source: manifest.SourcePacman, Status: "installed"},
		{Name: "waybar", Group: "desktop", Source: manifest.SourcePacman, Status: "missing"},
		{Name: "paru-bin", Group: "tools", Source: manifest.SourceAUR, Status: "missing"},
		{Name: "htop", Status: "drift"},
	}
	if len(got) != len(want) {
		t.Fatalf("Rows = %+v, want %d rows", got, len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Name != w.Name || g.Status != w.Status || g.Group != w.Group || (w.Source != "" && g.Source != w.Source) {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestRows_OrdersCatalogByGroupThenNameAndDriftLast(t *testing.T) {
	catalog := []manifest.Package{
		catalogPkg("zsh", "shell", manifest.SourcePacman),
		catalogPkg("waybar", "desktop", manifest.SourcePacman),
		catalogPkg("alacritty", "shell", manifest.SourcePacman),
		catalogPkg("hyprland", "desktop", manifest.SourcePacman),
	}

	got := Rows(map[string]bool{"zsh": true, "htop": true}, []string{"htop"}, catalog, nil, nil, nil)

	var names []string
	for _, r := range got {
		names = append(names, r.Name)
	}
	if want := []string{"hyprland", "waybar", "alacritty", "zsh", "htop"}; !slices.Equal(names, want) {
		t.Errorf("order = %v, want %v", names, want)
	}
}

func TestRows_DriverIgnoredAndPrerequisitePackagesGetNoRow(t *testing.T) {
	drivers := []manifest.Driver{{ID: "nvidia", VendorID: "10de", Packages: []string{"nvidia-utils"}}}
	explicit := []string{"nvidia-utils", "wget", "base-devel", "htop"}

	got := Rows(map[string]bool{}, explicit, nil, drivers, manifest.ParuPrerequisites, []string{"wget"})

	if len(got) != 1 || got[0].Name != "htop" || got[0].Status != "drift" {
		t.Errorf("Rows = %+v, want only htop as drift", got)
	}
}
