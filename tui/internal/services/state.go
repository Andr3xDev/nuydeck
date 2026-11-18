package services

import (
	"context"
	"errors"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// ErrNoUserBus means the user systemd bus is unreachable, so user state is unknown.
var ErrNoUserBus = errors.New("user systemd bus unavailable: no XDG_RUNTIME_DIR/bus")

// EnabledState and ActiveState hold systemd's raw value; the constants name the ones the TUI acts on.
type EnabledState string

const (
	EnabledEnabled  EnabledState = "enabled"
	EnabledDisabled EnabledState = "disabled"
	EnabledOther    EnabledState = "other"
)

type ActiveState string

const (
	ActiveActive   ActiveState = "active"
	ActiveInactive ActiveState = "inactive"
	ActiveFailed   ActiveState = "failed"
	ActiveOther    ActiveState = "other"
)

// UnitState is the read-only systemd state of one unit.
type UnitState struct {
	Found   bool
	Enabled EnabledState
	Active  ActiveState
	Masked  bool
}

// StateProbe reads the state of units in one scope without changing anything.
type StateProbe interface {
	Read(ctx context.Context, scope plan.Scope, units []string) (map[string]UnitState, error)
}

// ShownUnit is one block of `systemctl show` output.
type ShownUnit struct {
	ID    string
	State UnitState
}

// ParseShow parses `systemctl show` output: blank-line separated blocks keyed on Id, unknown keys ignored.
func ParseShow(text string) map[string]UnitState {
	states := map[string]UnitState{}
	for _, unit := range ParseShowBlocks(text) {
		states[unit.ID] = unit.State
	}
	return states
}

// ParseShowBlocks parses `systemctl show` output in block order, which follows the order the units were requested in.
func ParseShowBlocks(text string) []ShownUnit {
	var units []ShownUnit
	for _, block := range strings.Split(text, "\n\n") {
		props := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			if k, v, ok := strings.Cut(line, "="); ok {
				props[k] = v
			}
		}
		id := props["Id"]
		if id == "" {
			continue
		}
		load := props["LoadState"]
		state := UnitState{Found: load != "not-found", Masked: load == "masked"}
		state.Enabled = EnabledState(props["UnitFileState"])
		if state.Enabled == "" {
			state.Enabled = EnabledOther
		}
		state.Active = ActiveState(props["ActiveState"])
		if state.Active == "" {
			state.Active = ActiveOther
		}
		units = append(units, ShownUnit{ID: id, State: state})
	}
	return units
}
