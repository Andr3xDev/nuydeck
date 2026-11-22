package ui

import (
	"cmp"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
)

const (
	planningText    = "Planning..."
	confirmTitle    = "Confirm"
	confirmBoxWidth = 50
	defaultConfirm  = "Apply this plan?"
	emptyPlanText   = "Nothing to do."
)

func (f *flow) View(w, h int) string {
	switch f.stage {
	case stageOffer, stageConfirm, stageExtra:
		return f.dialogView(w, h)
	case stagePlanning:
		return f.svc.th.Muted.Render(planningText)
	case stagePreview:
		return strings.Join(f.previewLines(w, h), "\n")
	}
	return ""
}

// box draws the flow inside a screen's box of w x h cells.
func (f *flow) box(title string, w, h int) string {
	body := f.View(kit.ContentWidth(w), innerHeight(h))
	return screenBox(f.svc.th, title, strings.Split(body, "\n"), w, h)
}

func (f *flow) header(w int) []string {
	return styled(f.svc.th.Section, kit.Wrap(f.src.Title, w))
}

func (f *flow) previewLines(w, h int) []string {
	th := f.svc.th
	if f.err != nil {
		lines := styled(th.Section, kit.Wrap(f.src.Title+": cannot continue", w))
		return append(lines, styled(th.Err, kit.Wrap(f.err.Error(), w))...)
	}
	header := f.header(w)
	return append(header, f.body.Window(f.st.View(th, w, h-len(header)), h-len(header))...)
}

// dialogText is the question the current stage asks.
func (f *flow) dialogText() string {
	switch f.stage {
	case stageOffer:
		prompt, _ := f.st.Offer()
		return prompt
	case stageExtra:
		return f.built.ExtraConfirm
	}
	return cmp.Or(f.built.Confirm, defaultConfirm)
}

// dialogView draws the question as a compact box centered in the w x h panel.
func (f *flow) dialogView(w, h int) string {
	boxW := min(confirmBoxWidth, w)
	inner := kit.ContentWidth(boxW)
	header := f.header(inner)
	body := styled(f.svc.th.Warning, kit.Wrap(f.dialogText(), inner))
	room := max(h-kit.BorderCells-len(header), 0)
	var tail []string
	if len(body) > room && room > 1 {
		room--
		tail = []string{""}
	}
	shown := f.body.Window(body, room)
	if tail != nil {
		tail[0] = f.svc.th.Muted.Render(fmt.Sprintf("%d-%d/%d  j/k scroll", f.body.Offset+1, f.body.Offset+len(shown), len(body)))
	}
	lines := append(append(header, shown...), tail...)
	lines = lines[:min(len(lines), max(h-kit.BorderCells, 0))]
	box := kit.RenderBox(f.svc.th, confirmTitle, lines, boxW, len(lines)+kit.BorderCells, true)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// planStaged is the default preview of a bare plan: its steps, no offer.
type planStaged struct{ p plan.Plan }

func newPlanStaged(p plan.Plan) staged { return planStaged{p} }

func (s planStaged) View(th kit.Theme, w, _ int) []string {
	if s.p.Empty() {
		return styled(th.Muted, kit.Wrap(emptyPlanText, w))
	}
	var lines []string
	for _, step := range s.p.Steps {
		label, kind := plan.Describe(step)
		lines = append(lines, kit.Wrap(kind+"  "+label, w)...)
	}
	return lines
}

func (planStaged) Keys() []key.Binding            { return nil }
func (planStaged) Update(tea.KeyPressMsg) tea.Cmd { return nil }
func (s planStaged) CanApply() bool               { return !s.p.Empty() && len(s.p.Blocking) == 0 }
func (planStaged) Offer() (string, bool)          { return "", false }
func (s planStaged) Build(bool) plan.Plan         { return s.p }
