package domain

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
	PhaseServices
	PhaseTheme
	PhaseRecord
	PhaseReboot
)

// PlannedStep is a step tagged with its phase; a non-empty Skip is the reason it will not run.
type PlannedStep struct {
	Phase Phase
	Step  plan.Step
	Skip  string
}
