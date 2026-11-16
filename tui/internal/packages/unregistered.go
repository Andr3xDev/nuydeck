package packages

import (
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

// Unregistered lists explicit packages absent from the catalog, driver packages, paru prerequisites and the ignored list.
func Unregistered(explicit []string, catalog []manifest.Package, drivers []manifest.Driver, paruPrerequisites, ignored []string) []string {
	known := make(map[string]bool)
	for _, p := range catalog {
		known[p.Name] = true
	}
	for _, d := range drivers {
		for _, name := range d.Packages {
			known[name] = true
		}
	}
	for _, name := range slices.Concat(paruPrerequisites, ignored) {
		known[name] = true
	}
	out := []string{}
	for _, name := range explicit {
		if !known[name] {
			out = append(out, name)
		}
	}
	return out
}
