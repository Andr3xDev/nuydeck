package plan

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

type Scope string

const (
	ScopeSystem Scope = "system"
	ScopeUser   Scope = "user"
)

// Action values are the systemctl verbs.
type Action string

const (
	ActionEnable  Action = "enable"
	ActionDisable Action = "disable"
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
	ActionMask    Action = "mask"
	ActionUnmask  Action = "unmask"
)

var serviceActions = []Action{ActionEnable, ActionDisable, ActionStart, ActionStop, ActionRestart, ActionMask, ActionUnmask}

// unitName rejects a leading dash so a unit can never be read as a systemctl option.
var unitName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9:_.@-]*\.(service|timer)$`)

// ServiceActionStep runs one systemctl verb on a service or timer unit.
type ServiceActionStep struct {
	Unit   string
	Scope  Scope
	Action Action
}

// Label names the step for the log and the UI.
func (s ServiceActionStep) Label() string {
	return fmt.Sprintf("%s %s (%s)", s.Action, s.Unit, s.Scope)
}

// Command builds the systemctl CommandStep, refusing an unknown scope, action or unit name.
func (s ServiceActionStep) Command() (CommandStep, error) {
	if !slices.Contains(serviceActions, s.Action) {
		return CommandStep{}, fmt.Errorf("unknown service action %q", s.Action)
	}
	if !unitName.MatchString(s.Unit) {
		return CommandStep{}, fmt.Errorf("invalid unit name %q", s.Unit)
	}
	cmd := CommandStep{Label: s.Label()}
	switch s.Scope {
	case ScopeSystem:
		cmd.Argv = []string{"systemctl", string(s.Action), s.Unit}
		cmd.Sudo = true
	case ScopeUser:
		cmd.Argv = []string{"systemctl", "--user", string(s.Action), s.Unit}
	default:
		return CommandStep{}, errors.New("unknown service scope " + string(s.Scope))
	}
	return cmd, nil
}

// ConfirmStep pauses the run to ask the user a yes/no question; it performs no I/O.
type ConfirmStep struct {
	ID     string
	Prompt string
	// DeclineNotice is shown when the answer is "no" or the step this one depends on soft-failed.
	DeclineNotice string
}

func (ServiceActionStep) isStep() {}
func (ConfirmStep) isStep()       {}
