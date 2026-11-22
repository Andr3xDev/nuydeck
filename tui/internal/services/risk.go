package services

import (
	"fmt"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Risk returns the extra-confirm warning for an action, or "" when the normal confirm is enough.
func Risk(entry Entry, action plan.Action) string {
	const consequence = "can cut the network or end the graphical session"
	switch {
	case action == plan.ActionMask && entry.Critical:
		return fmt.Sprintf("%s is critical: masking it %s, since it blocks it from starting at all, even as a dependency, until it is unmasked.", entry.Unit, consequence)
	case action == plan.ActionMask:
		return fmt.Sprintf("Masking %s blocks it from starting at all, even as a dependency, until it is unmasked.", entry.Unit)
	case entry.Critical && action == plan.ActionDisable:
		return fmt.Sprintf("%s is critical: disabling it %s from the next boot.", entry.Unit, consequence)
	case entry.Critical && action == plan.ActionStop:
		return fmt.Sprintf("%s is critical: stopping it %s right now.", entry.Unit, consequence)
	}
	return ""
}
