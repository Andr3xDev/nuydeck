package selfupdate

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestPlan_RunsTheRepoUpdateScriptAsOneStreamedCommand(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "nuydeck")

	p := Plan(repo)

	if len(p.Steps) != 1 {
		t.Fatalf("steps = %d, want exactly one", len(p.Steps))
	}
	step, ok := p.Steps[0].(plan.CommandStep)
	if !ok {
		t.Fatalf("step = %T, want plan.CommandStep", p.Steps[0])
	}
	if want := []string{"bash", filepath.Join(repo, "update-tui.sh")}; !slices.Equal(step.Argv, want) {
		t.Errorf("argv = %v, want %v", step.Argv, want)
	}
	if step.Dir != repo {
		t.Errorf("dir = %q, want %q", step.Dir, repo)
	}
	if step.Sudo || step.UsesSudo || step.Interactive {
		t.Errorf("step = %+v, want no sudo and not interactive so output streams into the run pane", step)
	}
}
