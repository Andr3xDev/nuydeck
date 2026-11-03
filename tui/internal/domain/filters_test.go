package domain_test

import (
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func TestFiltersFor(t *testing.T) {
	spec := []domain.Exclude{
		{Root: "config", Pattern: "hypr/configs/monitors_generated.lua"},
		{Root: "config", Pattern: "quickshell/lucyna/legacy/monitors/data/identities.json"},
		{Root: "config", Pattern: "systemd/user/*.wants/"},
		{Root: "config", Pattern: "nuydeck/logs/"},
		{Root: "config", Pattern: "nuydeck/theme/"},
		{Root: "config", Pattern: "nvim/lazy-lock.json"},
	}
	tests := []struct {
		name      string
		entry     domain.Entry
		excludes  []domain.Exclude
		wantRules []string
		wantWhole bool
	}{
		{"pattern below the entry is anchored to it", entry("config", "hypr"), spec,
			[]string{"/configs/monitors_generated.lua"}, false},
		{"nested entry keeps the deeper remainder", entry("config", "quickshell/lucyna"), spec,
			[]string{"/legacy/monitors/data/identities.json"}, false},
		{"glob segment matches the entry segment", entry("config", "systemd/user"), spec,
			[]string{"/*.wants/"}, false},
		{"file entry beside a glob pattern is unaffected", entry("config", "systemd/user/lucyna-clipboard-wipe.service"), spec,
			nil, false},
		{"sibling entry gets no rule", entry("config", "nuydeck/lib"), spec, nil, false},
		{"entry in another root gets no rule", entry("home", "hypr"), spec, nil, false},
		{"entry equal to the pattern is excluded whole", entry("config", "nvim/lazy-lock.json"), spec, nil, true},
		{"trailing slash pattern excludes the equal dir entry", entry("config", "nuydeck/theme"), spec, nil, true},
		{"entry inside an excluded dir is excluded whole", entry("config", "nuydeck/logs/old"), spec, nil, true},
		{"several patterns keep manifest order", entry("config", "hypr"), []domain.Exclude{
			{Root: "config", Pattern: "hypr/b.lua"}, {Root: "config", Pattern: "hypr/a.lua"}},
			[]string{"/b.lua", "/a.lua"}, false},
		{"no excludes", entry("config", "hypr"), nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, whole := domain.FiltersFor(tt.entry, tt.excludes)
			if whole != tt.wantWhole {
				t.Errorf("wholeEntryExcluded = %v, want %v", whole, tt.wantWhole)
			}
			if !slices.Equal(got.Excludes, tt.wantRules) && !(len(got.Excludes) == 0 && len(tt.wantRules) == 0) {
				t.Errorf("rules = %q, want %q", got.Excludes, tt.wantRules)
			}
		})
	}
}
