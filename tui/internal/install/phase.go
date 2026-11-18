package install

import "github.com/Andr3xDev/nuydeck/tui/internal/plan"

// Phase is a stage of the guided install; the numeric order is the execution order.
type Phase int

const (
	PhasePrivilege Phase = iota
	PhasePacmanConf
	PhasePacman
	PhaseParuBootstrap
	PhaseAur
	PhaseSystem
	PhaseConfigs
	PhaseFonts
	PhaseEnv
	PhaseShell
	PhaseExtras
	PhaseServices
	PhaseTheme
	PhaseRecord
	PhaseReboot
)

// PlannedStep is a step tagged with its phase; a non-empty Skip is the reason it will not run.
// A non-empty After is the ID of the step it depends on: a ConfirmStep gates it (a "no" skips it), or an
// Optional step whose failure counts as a "no" answer on a ConfirmStep. A ConfirmStep is identified by its
// own ID; other steps set ID only when something depends on them.
// A failing Optional step is reported as a warning and does not stop the run.
type PlannedStep struct {
	Phase    Phase
	Step     plan.Step
	ID       string
	Skip     string
	After    string
	Optional bool
}
