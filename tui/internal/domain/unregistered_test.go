package domain_test

import (
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func TestCatalogUnregistered(t *testing.T) {
	catalog := []manifest.Package{{Name: "hyprland", Source: manifest.SourcePacman, Profiles: []string{"full"}}}
	drivers := []manifest.Driver{{ID: "nvidia", VendorID: "10de", Packages: []string{"nvidia-utils"}}}
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
