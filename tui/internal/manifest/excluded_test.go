package manifest_test

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func TestExcluded(t *testing.T) {
	tests := []struct {
		name  string
		rel   string
		isDir bool
		rules []string
		want  bool
	}{
		{"exact path", "a/b.lua", false, []string{"/a/b.lua"}, true},
		{"unrelated path", "a/c.lua", false, []string{"/a/b.lua"}, false},
		{"segment glob", "a/x.log", false, []string{"/a/*.log"}, true},
		{"star does not cross slash", "a/b/x.log", false, []string{"/a/*.log"}, false},
		{"path below a match is hit", "a/b/deep/f", false, []string{"/a/b"}, true},
		{"rule longer than path", "a", true, []string{"/a/b"}, false},
		{"dir-only rule hits a dir", "a/cache", true, []string{"/a/cache/"}, true},
		{"dir-only rule skips a file", "a/cache", false, []string{"/a/cache/"}, false},
		{"dir-only rule hits file below the dir", "a/cache/f", false, []string{"/a/cache/"}, true},
		{"no rules", "a", false, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := manifest.Excluded(tt.rel, tt.isDir, manifest.Filter{Excludes: tt.rules}); got != tt.want {
				t.Errorf("Excluded(%q, %v, %v) = %v, want %v", tt.rel, tt.isDir, tt.rules, got, tt.want)
			}
		})
	}
}
