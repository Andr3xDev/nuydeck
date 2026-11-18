package ui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type flowStage int

const (
	stageIdle flowStage = iota // the flow is at its entry: not started, declined or handed to a run
	stageSelect
	stagePlanning
	stagePreview
	stageOffer
	stageConfirm
	stageExtra
)

// staged is the preview a flow plans: what it shows, what it offers and the plan it builds.
type staged interface {
	View(th theme, w, h int) []string
	Keys() []key.Binding
	Update(tea.KeyPressMsg) tea.Cmd
	CanApply() bool
	// Offer returns the yes/no question asked before the confirm, if any.
	Offer() (prompt string, ok bool)
	// Build returns the plan to run; offer is the answer to Offer, false when none was asked.
	Build(offer bool) plan.Plan
}

// flowSource is what a screen supplies to run the flow: how to plan and how to run what was accepted.
type flowSource struct {
	Title       string
	SkipPreview bool // planning goes straight to the confirm
	Plan        func(ctx context.Context, selection []string) (staged, error)
	Run         func(plan.Plan) runRequest
}

// loader is a staged that reads data for its first view as soon as it is previewed.
type loader interface{ Load() tea.Cmd }

// msgHandler is a staged that receives the results of its own commands.
type msgHandler interface{ Handle(tea.Msg) tea.Cmd }

// nester is a staged with a sub-stage open (a diff or a popup): it gets every key, esc included.
type nester interface{ Nested() bool }

type plannedMsg struct {
	f   *flow
	gen int
	st  staged
	err error
}

// flow is the one write interaction: plan -> preview -> offer -> confirm -> extra -> run.
type flow struct {
	svc *services
	src flowSource

	stage flowStage
	gen   int // drops a plan that arrives after the flow moved on
	st    staged
	err   error
	built plan.Plan
	asked bool // the offer stage was shown, so esc at the confirm returns to it
	body  scroller
}

func newFlow(svc *services, src flowSource) *flow { return &flow{svc: svc, src: src} }

// start plans the flow's selection-less plan.
func (f *flow) start() tea.Cmd {
	f.reset()
	f.stage = stagePlanning
	gen, ctx, planFn := f.gen, f.svc.ctx, f.src.Plan
	return func() tea.Msg {
		st, err := planFn(ctx, nil)
		return plannedMsg{f: f, gen: gen, st: st, err: err}
	}
}

// reset goes back to the entry and drops any plan still being computed.
func (f *flow) reset() {
	f.gen++
	f.st, f.err, f.built, f.asked = nil, nil, plan.Plan{}, false
	f.body.reset()
	f.stage = stageIdle
}

// finished is true once the flow is at its entry and can be dropped.
func (f *flow) finished() bool { return f.stage == stageIdle }

// active is true while a flow exists and is past its entry; a nil flow is not active.
func (f *flow) active() bool { return f != nil && !f.finished() }

// Notices are the previewed plan's notices, shown in the shell's band until the flow ends.
func (f *flow) Notices() []string {
	if n, ok := f.st.(noticer); ok && f.active() && f.err == nil {
		return n.Notices()
	}
	return nil
}

func (f *flow) canApply() bool { return f.err == nil && f.st != nil && f.st.CanApply() }

func (f *flow) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case plannedMsg:
		return f.receive(msg)
	case tea.KeyPressMsg:
		return f.handleKey(msg)
	default:
		if h, ok := f.st.(msgHandler); ok {
			return h.Handle(msg)
		}
	}
	return nil
}

// receive drops a plan that is stale; one arriving while a run owns the screen also ends the flow.
// The returned command is the preview's first read, if it has one.
func (f *flow) receive(msg plannedMsg) tea.Cmd {
	if msg.f != f || msg.gen != f.gen || f.stage != stagePlanning {
		return nil
	}
	if f.svc.runIsActive() {
		f.reset()
		return nil
	}
	f.st, f.err = msg.st, msg.err
	f.body.reset()
	if f.src.SkipPreview && f.canApply() {
		f.openConfirm(false)
		return nil
	}
	f.stage = stagePreview
	if l, ok := f.st.(loader); ok && f.err == nil {
		return l.Load()
	}
	return nil
}

// nested is true while the preview has a sub-stage open.
func (f *flow) nested() bool {
	n, ok := f.st.(nester)
	return ok && f.stage == stagePreview && n.Nested()
}

func (f *flow) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := f.svc.keys
	if f.nested() {
		return f.st.Update(msg)
	}
	if key.Matches(msg, k.Back) {
		f.back()
		return nil
	}
	switch f.stage {
	case stagePreview:
		switch {
		case f.st != nil && key.Matches(msg, f.st.Keys()...):
			return f.st.Update(msg)
		case k.step(msg) != 0:
			f.body.by(k.step(msg))
		case key.Matches(msg, k.Confirm):
			f.accept()
		}
	case stageOffer:
		switch {
		case key.Matches(msg, k.Confirm, k.Yes):
			f.openConfirm(true)
		case key.Matches(msg, k.No):
			f.openConfirm(false)
		}
	case stageConfirm, stageExtra:
		if f.src.SkipPreview && k.step(msg) != 0 {
			f.body.by(k.step(msg))
			return nil
		}
		return f.answer(key.Matches(msg, k.Yes))
	}
	return nil
}

// back steps one stage toward the entry; planning cancels through the bumped gen.
func (f *flow) back() {
	switch {
	case f.stage == stageExtra:
		f.stage = stageConfirm
	case f.stage == stageConfirm && f.asked:
		f.stage = stageOffer
	case f.stage == stageConfirm && !f.src.SkipPreview, f.stage == stageOffer:
		f.stage = stagePreview
	default:
		f.reset()
	}
	f.body.reset()
}

// accept leaves the preview for the offer, or the confirm when nothing is offered.
func (f *flow) accept() {
	if !f.canApply() {
		return
	}
	if _, ok := f.st.Offer(); ok {
		f.stage, f.asked = stageOffer, true
		return
	}
	f.openConfirm(false)
}

func (f *flow) openConfirm(offer bool) {
	f.built = f.st.Build(offer)
	f.stage = stageConfirm
	f.body.reset()
}

// answer proceeds on yes; any other key declines and runs nothing.
func (f *flow) answer(yes bool) tea.Cmd {
	switch {
	case !yes:
		f.reset()
	case f.stage == stageConfirm && f.built.ExtraConfirm != "":
		f.stage = stageExtra
		f.body.reset()
	default:
		req := f.src.Run(f.built)
		f.reset()
		return func() tea.Msg { return req }
	}
	return nil
}

func (f *flow) Keys() []key.Binding {
	k := f.svc.keys
	if f.nested() {
		return f.st.Keys()
	}
	switch f.stage {
	case stagePlanning:
		return []key.Binding{k.Back}
	case stagePreview:
		var keys []key.Binding
		if f.canApply() {
			keys = append(keys, described(k.Confirm, "apply"))
		}
		if f.st != nil {
			keys = append(keys, f.st.Keys()...)
		}
		return append(keys, k.Back)
	case stageOffer:
		return []key.Binding{k.Yes, described(k.Confirm, "yes"), k.No, k.Back}
	case stageConfirm, stageExtra:
		if f.src.SkipPreview {
			return []key.Binding{k.Yes, k.Up, k.Down, described(k.Back, "back")}
		}
		return []key.Binding{k.Yes, described(k.Back, "back")}
	}
	return nil
}
