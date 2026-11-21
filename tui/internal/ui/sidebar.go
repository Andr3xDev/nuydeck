package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const sidebarTitle = "NUYDECK"

// renderSidebar draws the sidebar block, border included, width cells wide: the title row, the
// sections, then free space and the last pinned titles on the bottom rows. When the unpinned
// sections do not fit they scroll to keep the selection visible.
func renderSidebar(th theme, titles []string, selected, pinned int, panelFocus bool, width, height int) string {
	normal := th.secondary.Padding(0, 1)
	active := th.onAccent.Padding(0, 1)
	if !panelFocus {
		active = active.Bold(true)
	}

	cell := max(width-1, 1)
	rows := make([]string, len(titles))
	for i, title := range titles {
		style := normal
		if i == selected {
			style = active
		}
		rows[i] = style.Width(cell).MaxWidth(cell).Render(title)
	}

	split := len(rows) - pinned
	free := max(height-1-pinned, 0)
	body := windowAround(rows[:split], min(selected, max(split-1, 0)), free)
	lines := []string{th.section.Padding(0, 1).Width(cell).MaxWidth(cell).Render(sidebarTitle)}
	lines = append(lines, body...)
	lines = append(lines, make([]string, free-len(body))...)
	lines = append(lines, rows[split:]...)

	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).
		BorderStyle(lipgloss.NormalBorder()).BorderRight(true).BorderForeground(th.borderFor(!panelFocus).GetForeground()).
		Render(strings.Join(lines, "\n"))
}
