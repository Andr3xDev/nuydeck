package install_test

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

var (
	_ plan.Step = plan.CommandStep{}
	_ plan.Step = plan.WriteFileStep{}
	_ plan.Step = plan.SymlinkStep{}
	_ plan.Step = plan.WriteSecretsStep{}
	_ plan.Step = plan.AppendPackageStep{}
)

// The order comes from the spec, not from the manifest, so the enum itself must encode it.
func TestPhaseOrderMatchesSpec(t *testing.T) {
	want := []install.Phase{
		install.PhasePrivilege, install.PhasePacmanConf, install.PhasePacman, install.PhaseParuBootstrap,
		install.PhaseAur, install.PhaseSystem, install.PhaseConfigs, install.PhaseFonts, install.PhaseEnv,
		install.PhaseShell, install.PhaseServices, install.PhaseTheme, install.PhaseRecord, install.PhaseReboot,
	}
	for i, p := range want {
		if int(p) != i {
			t.Errorf("phase at spec position %d has value %d", i, int(p))
		}
	}
}

func TestPlannedStepAfterNamesTheGatingConfirmStep(t *testing.T) {
	gate := plan.ConfirmStep{ID: "spotify-opened", Prompt: "Did Spotify open?"}
	gated := install.PlannedStep{Phase: install.PhaseShell, Step: plan.CommandStep{Label: "spicetify backup apply"}, After: gate.ID}

	if gated.After != "spotify-opened" {
		t.Fatalf("After = %q, want the ConfirmStep ID", gated.After)
	}
}

func TestPlannedStepCarriesPhaseStepAndSkipReason(t *testing.T) {
	step := plan.CommandStep{Label: "install pacman packages (1)", Argv: []string{"pacman", "-Syu"}, Sudo: true}
	planned := install.PlannedStep{Phase: install.PhasePacman, Step: step, Skip: "already installed"}

	if planned.Phase != install.PhasePacman || planned.Skip != "already installed" {
		t.Fatalf("PlannedStep = %+v", planned)
	}
	var _ plan.Step = plan.WriteFileStep{}
	var _ plan.Step = plan.SymlinkStep{}
	var _ plan.Step = plan.WriteSecretsStep{}
	var _ plan.Step = plan.AppendPackageStep{}
}
