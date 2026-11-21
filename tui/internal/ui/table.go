package ui

import (
	"cmp"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	columnGap    = "  "
	cursorMarker = "  " // same width as the "> " listLines puts before a row
)

// column is one table column; a higher priority number is dropped first when the panel is narrow, 0 is never dropped.
type column struct {
	title    string
	priority int
}

type tableRow struct {
	cells []string
	muted bool
}

// tableLines aligns rows under a header whose columns fit width, dropping columns by priority.
// The header is returned apart so the caller draws it outside the scrolling window.
func tableLines(th theme, cols []column, rows []tableRow, cursor, width int) (header string, lines []string, cursorAt int) {
	shown := make([]int, len(cols))
	for i := range shown {
		shown[i] = i
	}
	cell := func(row tableRow, c int) string { return sanitizeLine(row.cells[c]) }
	widthOf := func(c int) int {
		w := lipgloss.Width(sanitizeLine(cols[c].title))
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
		drop := slices.MaxFunc(shown, func(a, b int) int { return cmp.Compare(cols[a].priority, cols[b].priority) })
		if cols[drop].priority == 0 {
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

	header = th.section.Render(truncate(cursorMarker+join(func(c int) string { return sanitizeLine(cols[c].title) }), width))
	listRows := make([]listRow, len(rows))
	for i, r := range rows {
		listRows[i] = listRow{text: join(func(c int) string { return cell(r, c) }), muted: r.muted}
	}
	lines, cursorAt = listLines(th, listRows, cursor, width)
	return header, lines, cursorAt
}
