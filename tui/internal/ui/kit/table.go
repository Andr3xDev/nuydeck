package kit

import (
	"cmp"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	columnGap    = "  "
	cursorMarker = "  " // same width as the "> " ListLines puts before a row
)

// Column is one table column; a higher Priority number is dropped first when the panel is narrow, 0 is never dropped.
type Column struct {
	Title    string
	Priority int
}

// TableRow is one table row; Muted rows render dimmed.
type TableRow struct {
	Cells []string
	Muted bool
}

// TableLines aligns rows under a header whose columns fit width, dropping columns by priority.
// The header is returned apart so the caller draws it outside the scrolling window.
func TableLines(th Theme, cols []Column, rows []TableRow, cursor, width int) (header string, lines []string, cursorAt int) {
	shown := make([]int, len(cols))
	for i := range shown {
		shown[i] = i
	}
	cell := func(row TableRow, c int) string { return SanitizeLine(row.Cells[c]) }
	widthOf := func(c int) int {
		w := lipgloss.Width(SanitizeLine(cols[c].Title))
		for _, r := range rows {
			w = max(w, lipgloss.Width(cell(r, c)))
		}
		return w
	}
	widths := make([]int, len(cols))
	for i := range cols {
		widths[i] = widthOf(i)
	}
	total := func() int {
		sum := lipgloss.Width(cursorMarker) + lipgloss.Width(columnGap)*(len(shown)-1)
		for _, c := range shown {
			sum += widths[c]
		}
		return sum
	}
	for total() > width && len(shown) > 1 {
		drop := slices.MaxFunc(shown, func(a, b int) int { return cmp.Compare(cols[a].Priority, cols[b].Priority) })
		if cols[drop].Priority == 0 {
			break
		}
		shown = slices.DeleteFunc(shown, func(c int) bool { return c == drop })
	}

	join := func(text func(c int) string) string {
		parts := make([]string, len(shown))
		for i, c := range shown {
			parts[i] = text(c)
			if i < len(shown)-1 {
				parts[i] += strings.Repeat(" ", widths[c]-lipgloss.Width(parts[i]))
			}
		}
		return strings.Join(parts, columnGap)
	}

	header = th.Section.Render(Truncate(cursorMarker+join(func(c int) string { return SanitizeLine(cols[c].Title) }), width))
	listRows := make([]ListRow, len(rows))
	for i, r := range rows {
		listRows[i] = ListRow{Text: join(func(c int) string { return cell(r, c) }), Muted: r.Muted}
	}
	lines, cursorAt = ListLines(th, listRows, cursor, width)
	return header, lines, cursorAt
}
