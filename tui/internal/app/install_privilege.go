package app

import "github.com/Andr3xDev/nuydeck/tui/internal/plan"

// sudoProbeStep checks the sudo timestamp without ever prompting.
func sudoProbeStep(label string) plan.CommandStep {
	return plan.CommandStep{Label: label, Argv: []string{"sudo", "-n", "-v"}, Quiet: true}
}

// PrivilegeSteps returns the interactive sudo handoff followed by its verification probe.
func PrivilegeSteps() []plan.Step {
	return []plan.Step{
		plan.CommandStep{Label: "authenticate sudo", Argv: []string{"sudo", "-v"}, Interactive: true},
		sudoProbeStep("verify sudo credentials"),
	}
}
