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
		text := prefix + row.text
		for _, b := range row.badges {
			text += " [" + b + "]"
		}
		for _, l := range wrap(text, width) {
			l = strings.TrimRight(l, " ")
			switch {
			case i == cursor:
				l = t.selected.Render(l + strings.Repeat(" ", max(width-lipgloss.Width(l), 0)))
			case row.muted:
				l = t.muted.Render(l)
			case row.header:
				l = t.section.Render(l)
			default:
				// Badge text can wrap across lines; only a badge left whole on a line is styled.
				for _, b := range row.badges {
					l = strings.ReplaceAll(l, "["+b+"]", t.warning.Render("["+b+"]"))
				}
				l = t.plain.Render(l)
			}
			lines = append(lines, l)
		}
	}
	return lines, cursorAt
}
