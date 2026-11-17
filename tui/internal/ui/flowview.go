package ui

import (
	"cmp"
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
	lines := append(header, f.body.window(body, max(h-borderCells-len(header), 0))...)
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
