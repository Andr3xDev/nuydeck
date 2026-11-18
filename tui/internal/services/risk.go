package services

import (
	"fmt"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Risk returns the extra-confirm warning for an action, or "" when the normal confirm is enough.
func Risk(entry Entry, action plan.Action) string {
	switch {
	case action == plan.ActionMask:
		return fmt.Sprintf("Masking %s blocks it from starting at all, even as a dependency, until it is unmasked.", entry.Unit)
	case entry.Critical && (action == plan.ActionStop || action == plan.ActionDisable):
		return fmt.Sprintf("%s is critical: to %s it can cut the network or end the graphical session.", entry.Unit, action)
	}
	return ""
}
