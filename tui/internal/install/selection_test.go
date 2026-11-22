package install

import (
	"maps"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func TestDefaultSelection(t *testing.T) {
	catalog := []manifest.Package{
		{Name: "base", Profiles: []string{manifest.ProfileMinimal, manifest.ProfileFull}},
		{Name: "extras", Profiles: []string{manifest.ProfileFull}},
		{Name: "orphan"},
	}
	want := map[string]bool{"base": true, "extras": false, "orphan": false}

	// The wizard only reaches the selection page for the custom profile and seeds it from minimal.
	for _, profile := range []string{ProfileCustom, manifest.ProfileMinimal} {
		t.Run(profile, func(t *testing.T) {
			if got := DefaultSelection(profile, catalog); !maps.Equal(got, want) {
				t.Errorf("DefaultSelection(%q) = %v, want %v", profile, got, want)
			}
		})
	}

	t.Run("empty catalog gives an empty non-nil map", func(t *testing.T) {
		if got := DefaultSelection(ProfileCustom, nil); got == nil || len(got) != 0 {
			t.Errorf("DefaultSelection = %v, want empty non-nil map", got)
		}
	})
}
