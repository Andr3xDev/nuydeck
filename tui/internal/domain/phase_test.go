package domain_test

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

var (
	_ domain.Step = domain.CommandStep{}
	_ domain.Step = domain.WriteFileStep{}
	_ domain.Step = domain.SymlinkStep{}
	_ domain.Step = domain.WriteSecretsStep{}
	_ domain.Step = domain.AppendPackageStep{}
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
	step := domain.CommandStep{Label: "install pacman packages (1)", Argv: []string{"pacman", "-Syu"}, Sudo: true}
	planned := domain.PlannedStep{Phase: domain.PhasePacman, Step: step, Skip: "already installed"}

	if planned.Phase != domain.PhasePacman || planned.Skip != "already installed" {
		t.Fatalf("PlannedStep = %+v", planned)
	}
	var _ domain.Step = domain.WriteFileStep{}
	var _ domain.Step = domain.SymlinkStep{}
	var _ domain.Step = domain.WriteSecretsStep{}
	var _ domain.Step = domain.AppendPackageStep{}
}
