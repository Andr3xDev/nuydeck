package plan

// SudoProbeStep checks the sudo timestamp without ever prompting.
func SudoProbeStep(label string) CommandStep {
	return CommandStep{Label: label, Argv: []string{"sudo", "-n", "-v"}, Quiet: true}
}

// PrivilegeSteps returns the interactive sudo handoff followed by its verification probe.
func PrivilegeSteps() []Step {
	return []Step{
		CommandStep{Label: "authenticate sudo", Argv: []string{"sudo", "-v"}, Interactive: true},
		SudoProbeStep("verify sudo credentials"),
	}
}
