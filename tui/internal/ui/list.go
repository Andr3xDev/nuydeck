package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type listRow struct {
	text  string
	muted bool
}

// listLines wraps every row to width and highlights the cursor row.
// It also returns the line where the cursor row starts, for windowAround.
func listLines(t theme, rows []listRow, cursor, width int) ([]string, int) {
	var lines []string
	cursorAt := 0
	for i, row := range rows {
		prefix := "  "
		if i == cursor {
			cursorAt, prefix = len(lines), "> "
		}
		for _, l := range wrap(prefix+row.text, width) {
			l = strings.TrimRight(l, " ")
			switch {
			case i == cursor:
				l = t.selected.Render(l + strings.Repeat(" ", max(width-lipgloss.Width(l), 0)))
			case row.muted:
				l = t.muted.Render(l)
			}
			lines = append(lines, l)
		}
	}
	return lines, cursorAt
}
