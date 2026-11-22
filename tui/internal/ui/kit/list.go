package kit

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// ListRow is one list entry. Badges render as " [badge]" after the text, in the warning style;
// Header rows are group titles.
type ListRow struct {
	Text   string
	Badges []string
	Muted  bool
	Header bool
}

// ListLines draws every row on one line, cut to width, and highlights the cursor row.
// It also returns the line of the cursor row, for WindowAround.
func ListLines(t Theme, rows []ListRow, cursor, width int) ([]string, int) {
	var lines []string
	cursorAt := 0
	for i, row := range rows {
		prefix := "  "
		if i == cursor {
			cursorAt, prefix = len(lines), "> "
		}
		var badges string
		for _, b := range row.Badges {
			badges += " [" + b + "]"
		}
		// The text gives way to the badges, which carry the row's state.
		text := prefix + Truncate(SanitizeLine(row.Text), max(width-lipgloss.Width(prefix+badges), 0)) + badges
		l := strings.TrimRight(Truncate(text, width), " ")
		switch {
		case i == cursor:
			l = t.Selected.Render(l + strings.Repeat(" ", max(width-lipgloss.Width(l), 0)))
		case row.Muted:
			l = t.Muted.Render(l)
		case row.Header:
			l = t.Section.Render(l)
		default:
			for _, b := range row.Badges {
				l = strings.ReplaceAll(l, "["+b+"]", t.Warning.Render("["+b+"]"))
			}
			l = t.Plain.Render(l)
		}
		lines = append(lines, l)
	}
	return lines, cursorAt
}
