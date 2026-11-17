package ui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// keybar renders the bindings valid in the current focus on the left, cut to fit, and the pinned
// bindings with the dry-run badge on the right, which never get cut.
func keybar(t theme, h help.Model, bindings, pinned []key.Binding, dryRun bool, width int) string {
	right := h.ShortHelpView(pinned)
	if dryRun {
		right += " " + t.badge.Render(dryRunBadge)
	}
	left := fitBindings(h, bindings, width-lipgloss.Width(right)-1)
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	return left + strings.Repeat(" ", gap) + right
}

// fitBindings keeps the longest prefix of bindings that fits room, then an ellipsis when it fits too.
// help.Model's own width cut keeps every item once the first one overflows within two cells of the limit.
func fitBindings(h help.Model, bindings []key.Binding, room int) string {
	tail := " " + h.Styles.Ellipsis.Inline(true).Render(h.Ellipsis)
	for n := len(bindings); n > 0; n-- {
		view := h.ShortHelpView(bindings[:n])
		if lipgloss.Width(view) > room {
			continue
		}
		if n < len(bindings) && lipgloss.Width(view)+lipgloss.Width(tail) <= room {
			view += tail
		}
		return view
	}
	return ""
}
