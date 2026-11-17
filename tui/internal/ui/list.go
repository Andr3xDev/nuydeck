package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// listRow is one list entry. badges render as " [badge]" after the text, in the warning style;
// header rows are group titles.
type listRow struct {
	text   string
	badges []string
	muted  bool
	header bool
}

// listLines draws every row on one line, cut to width, and highlights the cursor row.
// It also returns the line of the cursor row, for windowAround.
func listLines(t theme, rows []listRow, cursor, width int) ([]string, int) {
	var lines []string
	cursorAt := 0
	for i, row := range rows {
		prefix := "  "
		if i == cursor {
			cursorAt, prefix = len(lines), "> "
		}
		var badges string
		for _, b := range row.badges {
			badges += " [" + b + "]"
		}
		// The text gives way to the badges, which carry the row's state.
		text := prefix + truncate(sanitizeLine(row.text), max(width-lipgloss.Width(prefix+badges), 0)) + badges
		l := strings.TrimRight(truncate(text, width), " ")
		switch {
		case i == cursor:
			l = t.selected.Render(l + strings.Repeat(" ", max(width-lipgloss.Width(l), 0)))
		case row.muted:
			l = t.muted.Render(l)
		case row.header:
			l = t.section.Render(l)
		default:
			for _, b := range row.badges {
				l = strings.ReplaceAll(l, "["+b+"]", t.warning.Render("["+b+"]"))
			}
			l = t.plain.Render(l)
		}
		lines = append(lines, l)
	}
	return lines, cursorAt
}
