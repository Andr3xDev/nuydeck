package services

import (
	"context"
	"errors"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// ErrNoUserBus means the user systemd bus is unreachable, so user state is unknown.
var ErrNoUserBus = errors.New("user systemd bus unavailable: no XDG_RUNTIME_DIR/bus")

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

// ParseShow parses `systemctl show` output: blank-line separated blocks keyed on Id, unknown keys ignored.
func ParseShow(text string) map[string]UnitState {
	states := map[string]UnitState{}
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
		switch props["UnitFileState"] {
		case "enabled":
			state.Enabled = EnabledEnabled
		case "disabled":
			state.Enabled = EnabledDisabled
		default:
			state.Enabled = EnabledOther
		}
		switch props["ActiveState"] {
		case "active":
			state.Active = ActiveActive
		case "inactive":
			state.Active = ActiveInactive
		case "failed":
			state.Active = ActiveFailed
		default:
			state.Active = ActiveOther
		}
		states[id] = state
	}
	return states
}
