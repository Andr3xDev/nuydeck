package services

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

var (
	ErrNotManaged   = errors.New("unit is not in the services catalog")
	ErrUnitNotFound = errors.New("unit is not installed")
)

// Request names one action on one catalog unit.
type Request struct {
	Unit   string
	Scope  plan.Scope
	Action plan.Action
}

func find(catalog []Entry, unit string, scope plan.Scope) (Entry, error) {
	i := slices.IndexFunc(catalog, func(e Entry) bool { return e.Unit == unit && e.Scope == scope })
	if i < 0 {
		return Entry{}, fmt.Errorf("%w: %s (%s)", ErrNotManaged, unit, scope)
	}
	return catalog[i], nil
}

// PlanAction builds the plan for one action; the catalog is the closed set of reachable units.
func PlanAction(catalog []Entry, req Request, state UnitState, sudoCached bool) (plan.Plan, error) {
	entry, err := find(catalog, req.Unit, req.Scope)
	if err != nil {
		return plan.Plan{}, err
	}
	if !state.Found {
		return plan.Plan{}, fmt.Errorf("%w: %s", ErrUnitNotFound, req.Unit)
	}
	var steps []plan.Step
	if req.Scope == plan.ScopeSystem && !sudoCached {
		steps = plan.PrivilegeSteps()
	}
	steps = append(steps, plan.ServiceActionStep{Unit: req.Unit, Scope: req.Scope, Action: req.Action})
	return plan.Plan{
		Title:        fmt.Sprintf("%s %s", req.Action, req.Unit),
		Steps:        steps,
		ExtraConfirm: Risk(entry, req.Action),
	}, nil
}

// LogsCommand returns the journalctl pager handoff for a unit; a unit with an invalid name is refused.
func LogsCommand(entry Entry) (*exec.Cmd, error) {
	if _, err := (plan.ServiceActionStep{Unit: entry.Unit, Scope: entry.Scope, Action: plan.ActionStart}).Command(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotManaged, err)
	}
	args := []string{"-e"}
	if entry.Scope == plan.ScopeUser {
		args = append(args, "--user")
	}
	return exec.Command("journalctl", append(args, "-u", entry.Unit)...), nil
}
