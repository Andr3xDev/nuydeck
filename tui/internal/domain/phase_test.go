package domain_test

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
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
	want := []domain.Phase{
		domain.PhasePrivilege, domain.PhasePacmanConf, domain.PhasePacman, domain.PhaseParuBootstrap,
		domain.PhaseAur, domain.PhaseSystem, domain.PhaseConfigs, domain.PhaseFonts, domain.PhaseEnv,
		domain.PhaseShell, domain.PhaseServices, domain.PhaseTheme, domain.PhaseRecord, domain.PhaseReboot,
	}
	for i, p := range want {
		if int(p) != i {
			t.Errorf("phase at spec position %d has value %d", i, int(p))
		}
	}
}

func TestPlannedStepCarriesPhaseStepAndSkipReason(t *testing.T) {
	step := plan.CommandStep{Label: "install pacman packages (1)", Argv: []string{"pacman", "-Syu"}, Sudo: true}
	planned := domain.PlannedStep{Phase: domain.PhasePacman, Step: step, Skip: "already installed"}

	if planned.Phase != domain.PhasePacman || planned.Skip != "already installed" {
		t.Fatalf("PlannedStep = %+v", planned)
	}
	var _ plan.Step = plan.WriteFileStep{}
	var _ plan.Step = plan.SymlinkStep{}
	var _ plan.Step = plan.WriteSecretsStep{}
	var _ plan.Step = plan.AppendPackageStep{}
}
