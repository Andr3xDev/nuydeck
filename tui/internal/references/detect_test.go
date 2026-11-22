package references_test

import (
	"errors"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/references"
)

func TestDetect(t *testing.T) {
	const home = "/home/u"
	tests := []struct {
		name    string
		ref     references.Reference
		hasCmd  bool
		kind    plan.NodeKind
		kindErr error
		want    references.Status
	}{
		{"no detection", references.Reference{}, true, plan.KindDir, nil, references.StatusUnknown},
		{"binary present", references.Reference{DetectBinary: "pacman"}, true, plan.KindNone, nil, references.StatusPresent},
		{"binary absent", references.Reference{DetectBinary: "pacman"}, false, plan.KindNone, nil, references.StatusAbsent},
		{"path present", references.Reference{DetectPath: "~/dev"}, false, plan.KindDir, nil, references.StatusPresent},
		{"path absent", references.Reference{DetectPath: "~/dev"}, true, plan.KindNone, nil, references.StatusAbsent},
		{"either present", references.Reference{DetectBinary: "x", DetectPath: "~/dev"}, false, plan.KindDir, nil, references.StatusPresent},
		{"all absent", references.Reference{DetectBinary: "x", DetectPath: "~/dev"}, false, plan.KindNone, nil, references.StatusAbsent},
		{"probe error", references.Reference{DetectPath: "~/dev"}, false, plan.KindNone, errors.New("denied"), references.StatusUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathKind := func(string) (plan.NodeKind, error) { return tt.kind, tt.kindErr }
			if got := references.Detect(tt.ref, func(string) bool { return tt.hasCmd }, pathKind, home); got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetect_ExpandsLeadingTildeToHome(t *testing.T) {
	var probed string
	pathKind := func(p string) (plan.NodeKind, error) { probed = p; return plan.KindDir, nil }
	references.Detect(references.Reference{DetectPath: "~/dev/x"}, nil, pathKind, "/home/u")
	if probed != "/home/u/dev/x" {
		t.Errorf("probed %q, want /home/u/dev/x", probed)
	}
}
