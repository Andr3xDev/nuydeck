package kit

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

const helpTitle = "Help"

// HelpPopup sizes a Help box to its content, clamped to w x maxH.
func HelpPopup(t Theme, model help.Model, groups [][]key.Binding, w, maxH int) Popup {
	model.SetWidth(ContentWidth(w))
	lines := strings.Split(model.FullHelpView(groups), "\n")
	widest := 0
	for _, l := range lines {
		widest = max(widest, lipgloss.Width(l))
	}
	// +1 keeps a blank cell between the content and the right border.
	popupW := min(widest+BorderCells+boxPadding+1, w)
	popupH := min(len(lines)+BorderCells, maxH)
	for i, l := range lines {
		lines[i] = lipgloss.NewStyle().MaxWidth(ContentWidth(popupW)).Render(l)
	}
	return Popup{box: RenderBox(t, helpTitle, lines, popupW, popupH, true), w: popupW, h: popupH}
}

// Popup is a rendered box with its size, ready to be centered over a screen.
type Popup struct {
	box  string
	w, h int
}

// OverlayCentered draws the popup over base, centered in a w x h area.
func OverlayCentered(base string, p Popup, w, h int) string {
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(p.box).X((w-p.w)/2).Y((h-p.h)/2).Z(1),
	).Render()
}
