package install

import (
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

// DefaultSelection seeds the package selection with the minimal profile; every catalog package gets an entry.
func DefaultSelection(_ string, catalog []manifest.Package) map[string]bool {
	selected := make(map[string]bool, len(catalog))
	for _, p := range catalog {
		selected[p.Name] = slices.Contains(p.Profiles, manifest.ProfileMinimal)
	}
	return selected
}
