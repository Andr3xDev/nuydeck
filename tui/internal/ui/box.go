package ui

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
)

const (
	borderCells = 2 // left + right (or top + bottom) border
	boxPadding  = 1 // blank cell between the left border and the content
)

// contentWidth is the width available to lines passed to renderBox for a box w cells wide.
func contentWidth(w int) int { return max(w-borderCells-boxPadding, 0) }

// renderBox draws a rounded border w x h with the title embedded in the top border.
// lines must already fit contentWidth(w); extra lines are cut, missing ones padded.
func renderBox(t theme, title string, lines []string, w, h int, focused bool) string {
	border := t.border(focused)
	innerW, innerH := w-borderCells, h-borderCells
	b := lipgloss.RoundedBorder()

	// head adds 2 spaces and the top row 3 corner/dash cells around the title.
	title = lipgloss.NewStyle().MaxWidth(max(innerW-3, 0)).Render(title)
	head := " " + t.title.Render(title) + " "
	fill := max(innerW-lipgloss.Width(head)-1, 0)
	top := border.Render(b.TopLeft+b.Top) + head + border.Render(strings.Repeat(b.Top, fill)+b.TopRight)

	rows := []string{top}
	for i := range innerH {
		line := ""
		if i < len(lines) {
			line = strings.Repeat(" ", boxPadding) + lines[i]
		}
		pad := strings.Repeat(" ", max(innerW-lipgloss.Width(line), 0))
		rows = append(rows, border.Render(b.Left)+line+pad+border.Render(b.Right))
	}
	rows = append(rows, border.Render(b.BottomLeft+strings.Repeat(b.Bottom, innerW)+b.BottomRight))
	return strings.Join(rows, "\n")
}

// truncate cuts plain text to width cells, ending with an ellipsis when cut.
func truncate(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	if width < 1 {
		return ""
	}
	return string(runes) + "…"
}

// sanitize drops control characters (ESC, C0, C1) except newline: file-derived text must not
// reach the terminal with live sequences such as OSC 8 links or DCS/APC strings. Bidi controls
// (U+202E, U+2066...) are category Cf, which IsControl misses, and would reorder the displayed path.
func sanitize(text string) string {
	return strings.Map(func(r rune) rune {
		if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r)) {
			return -1
		}
		return r
	}, text)
}

// sanitizeLine is sanitize for single-line text: newlines are dropped too.
func sanitizeLine(text string) string {
	return strings.ReplaceAll(sanitize(text), "\n", "")
}

// wrap sanitizes and wraps plain, unstyled text; every file-derived string goes through it.
func wrap(text string, width int) []string {
	return strings.Split(lipgloss.NewStyle().Width(max(width, 1)).Render(sanitize(text)), "\n")
}
