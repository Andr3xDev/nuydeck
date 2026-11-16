package ui

import (
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func cursorMark(on bool) string {
	if on {
		return "> "
	}
	return "  "
}

func checkbox(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func directionLabel(d plan.Direction) string {
	switch d {
	case plan.Pull:
		return "Pull"
	case plan.Restore:
		return "Restore"
	}
	return "Push"
}

func changeLabel(t plan.ChangeType) string {
	switch t {
	case plan.Create:
		return "create"
	case plan.Delete:
		return "delete"
	}
	return "update"
}

func badges(it cfgsync.PlanItem) []string {
	var out []string
	switch it.Change.Warning {
	case plan.WarnAbsoluteLink:
		out = append(out, "absolute link")
	case plan.WarnLeavesManagedSet:
		out = append(out, "leaves managed set")
	}
	if it.Blocked {
		out = append(out, "blocked")
	}
	return out
}

func timeText(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(timeLayout)
}
