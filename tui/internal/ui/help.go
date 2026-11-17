package ui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

const helpTitle = "Help"

// helpPopup sizes a Help box to its content, clamped to w x maxH.
func helpPopup(t theme, model help.Model, groups [][]key.Binding, w, maxH int) popup {
	model.SetWidth(contentWidth(w))
	lines := strings.Split(model.FullHelpView(groups), "\n")
	widest := 0
	for _, l := range lines {
		widest = max(widest, lipgloss.Width(l))
	}
	// +1 keeps a blank cell between the content and the right border.
	popupW := min(widest+borderCells+boxPadding+1, w)
	popupH := min(len(lines)+borderCells, maxH)
	for i, l := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(contentWidth(popupW)).Render(l)
	}
	return popup{box: renderBox(t, helpTitle, lines, popupW, popupH, true), w: popupW, h: popupH}
}

type popup struct {
	box  string
	w, h int
}

func overlayCentered(base string, p popup, w, h int) string {
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(p.box).X((w-p.w)/2).Y((h-p.h)/2).Z(1),
	).Render()
}
