package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const sidebarWidth = 20

func renderSidebar(th theme, titles []string, selected int, panelFocus bool, height int) string {
	normal := th.secondary.Padding(0, 1)
	active := th.onAccent.Padding(0, 1)
	if !panelFocus {
		active = active.Bold(true)
	}

	lines := make([]string, len(titles))
	for i, title := range titles {
		style := normal
		if i == selected {
			style = active
		}
		lines[i] = style.Width(sidebarWidth).Render(title)
	}
	return lipgloss.NewStyle().Width(sidebarWidth).Height(height).
		BorderStyle(lipgloss.NormalBorder()).BorderRight(true).BorderForeground(th.border.GetForeground()).
		Render(strings.Join(lines, "\n"))
}
