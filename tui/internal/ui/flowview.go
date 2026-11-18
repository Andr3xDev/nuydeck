package ui

import (
	"cmp"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
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
		return f.svc.th.muted.Render(planningText)
	case stagePreview:
		return strings.Join(f.previewLines(w, h), "\n")
	}
	return ""
}

// box draws the flow inside a screen's box of w x h cells.
func (f *flow) box(title string, w, h int) string {
	body := f.View(contentWidth(w), innerHeight(h))
	return screenBox(f.svc.th, title, strings.Split(body, "\n"), w, h)
}

func (f *flow) header(w int) []string {
	return styled(f.svc.th.section, wrap(f.src.Title, w))
}

func (f *flow) previewLines(w, h int) []string {
	th := f.svc.th
	if f.err != nil {
		lines := styled(th.section, wrap(f.src.Title+": cannot continue", w))
		return append(lines, styled(th.err, wrap(f.err.Error(), w))...)
	}
	header := f.header(w)
	return append(header, f.body.window(f.st.View(th, w, h-len(header)), h-len(header))...)
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
	inner := contentWidth(boxW)
	header := f.header(inner)
	body := styled(f.svc.th.warning, wrap(f.dialogText(), inner))
	room := max(h-borderCells-len(header), 0)
	var tail []string
	if len(body) > room && room > 1 {
		room--
		tail = []string{""}
	}
	shown := f.body.window(body, room)
	if tail != nil {
		tail[0] = f.svc.th.muted.Render(fmt.Sprintf("%d-%d/%d  j/k scroll", f.body.offset+1, f.body.offset+len(shown), len(body)))
	}
	lines := append(append(header, shown...), tail...)
	lines = lines[:min(len(lines), max(h-borderCells, 0))]
	box := renderBox(f.svc.th, confirmTitle, lines, boxW, len(lines)+borderCells, true)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// planStaged is the default preview of a bare plan: its steps, no offer.
type planStaged struct{ p plan.Plan }

func newPlanStaged(p plan.Plan) staged { return planStaged{p} }

func (s planStaged) View(th theme, w, _ int) []string {
	if s.p.Empty() {
		return styled(th.muted, wrap(emptyPlanText, w))
	}
	var lines []string
	for _, step := range s.p.Steps {
		label, kind := plan.Describe(step)
		lines = append(lines, wrap(kind+"  "+label, w)...)
	}
	return lines
}

func (planStaged) Keys() []key.Binding            { return nil }
func (planStaged) Update(tea.KeyPressMsg) tea.Cmd { return nil }
func (s planStaged) CanApply() bool               { return !s.p.Empty() && len(s.p.Blocking) == 0 }
func (planStaged) Offer() (string, bool)          { return "", false }
func (s planStaged) Build(bool) plan.Plan         { return s.p }
