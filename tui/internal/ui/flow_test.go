package ui

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// fakeStaged is a preview whose offer and built plan the test controls; offers records what Build received.
type fakeStaged struct {
	cannotApply bool
	offer       string
	built       plan.Plan
	offers      []bool
}

func (s *fakeStaged) View(theme, int, int) []string   { return []string{"fake preview"} }
func (s *fakeStaged) Keys() []key.Binding             { return nil }
func (s *fakeStaged) Update(tea.KeyPressMsg) tea.Cmd  { return nil }
func (s *fakeStaged) CanApply() bool                  { return !s.cannotApply }
func (s *fakeStaged) Offer() (prompt string, ok bool) { return s.offer, s.offer != "" }
func (s *fakeStaged) Build(offer bool) plan.Plan      { s.offers = append(s.offers, offer); return s.built }

// flowHarness owns the flow under test and the plans its Run hook received.
type flowHarness struct {
	f     *flow
	ran   []plan.Plan
	state *fakeStaged
}

func newTestFlow(t *testing.T, st *fakeStaged, skipPreview bool) *flowHarness {
	t.Helper()
	h := &flowHarness{state: st}
	h.f = newFlow(testServices(Deps{}), flowSource{
		Title:       "Test",
		SkipPreview: skipPreview,
		Plan:        func(context.Context, []string) (staged, error) { return st, nil },
		Run: func(p plan.Plan) runRequest {
			h.ran = append(h.ran, p)
			return runRequest{title: p.Title}
		},
	})
	return h
}

// planned starts the flow and delivers its plan the way Root would.
func planned(f *flow) { f.Update(f.start()()) }

func isRunRequest(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(runRequest)
	return ok
}

// pressKeys feeds keys in order and returns the command of the last one.
func pressKeys(f *flow, keys ...tea.KeyPressMsg) (cmd tea.Cmd) {
	for _, k := range keys {
		cmd = f.Update(k)
	}
	return cmd
}

func withExtra(confirm, extra string) *fakeStaged {
	return &fakeStaged{built: plan.Plan{Title: "built", Confirm: confirm, ExtraConfirm: extra}}
}

func TestFlowStalePlanAfterEscAndRestartIsDropped(t *testing.T) {
	h := newTestFlow(t, withExtra("", ""), false)
	stale := h.f.start()()
	h.f.Update(keyEsc)
	h.f.start()

	h.f.Update(stale)

	if h.f.stage != stagePlanning {
		t.Fatalf("stage = %v, want planning: the superseded plan must be dropped", h.f.stage)
	}
}

func TestFlowEscInPlanningCancelsAndTheLatePlanIsIgnored(t *testing.T) {
	h := newTestFlow(t, withExtra("", ""), false)
	late := h.f.start()()

	h.f.Update(keyEsc)
	h.f.Update(late)

	if !h.f.finished() {
		t.Fatalf("stage = %v, want the flow back at its entry, plan ignored", h.f.stage)
	}
}

// flowScreen puts a flow in a Root section so messages reach it as they do in the program.
type flowScreen struct{ f *flow }

func (s *flowScreen) Title() string        { return "Flow" }
func (s *flowScreen) Enter() tea.Cmd       { return nil }
func (s *flowScreen) View(int, int) string { return "" }
func (s *flowScreen) Keys() []key.Binding  { return nil }
func (s *flowScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	return s, s.f.Update(msg)
}

func TestFlowPlanIsIgnoredWhileARunIsActive(t *testing.T) {
	root := NewRoot("", false, Deps{})
	t.Cleanup(root.cancel)
	st := withExtra("", "")
	f := newFlow(root.svc, flowSource{
		Title: "Test",
		Plan:  func(context.Context, []string) (staged, error) { return st, nil },
		Run:   func(p plan.Plan) runRequest { return runRequest{title: p.Title} },
	})
	root.sections = []section{{title: "Flow", screen: &flowScreen{f}}}
	result := f.start()()
	root.Update(runRequest{title: "busy", work: func(ctx context.Context, _ func(plan.Event)) runEnd {
		<-ctx.Done()
		return runEnd{}
	}})
	if root.run == nil {
		t.Fatal("run did not open")
	}

	root.Update(result)

	if f.stage != stagePlanning {
		t.Fatalf("stage = %v, want planning: a plan arriving during a run must not change the flow", f.stage)
	}
}

func TestFlowExtraStageOnlyWhenThePlanCarriesExtraConfirm(t *testing.T) {
	t.Run("without", func(t *testing.T) {
		h := newTestFlow(t, withExtra("sure?", ""), false)
		planned(h.f)

		cmd := pressKeys(h.f, keyEnter, keyPress('y'))

		if !isRunRequest(cmd) {
			t.Fatalf("y at confirm: stage = %v, want a runRequest and no extra stage", h.f.stage)
		}
	})
	t.Run("with", func(t *testing.T) {
		h := newTestFlow(t, withExtra("sure?", "really?"), false)
		planned(h.f)

		cmd := pressKeys(h.f, keyEnter, keyPress('y'))
		if isRunRequest(cmd) || h.f.stage != stageExtra {
			t.Fatalf("y at confirm: stage = %v, want the extra stage and no run", h.f.stage)
		}

		if !isRunRequest(h.f.Update(keyPress('y'))) {
			t.Fatal("y at extra must produce a runRequest")
		}
	})
}

func TestFlowAnyKeyButYDeclinesAndRunsNothing(t *testing.T) {
	declines := map[string]tea.KeyPressMsg{"n": keyPress('n'), "x": keyPress('x'), "enter": keyEnter, "space": keyPress(tea.KeySpace)}
	for _, atExtra := range []bool{false, true} {
		for name, k := range declines {
			t.Run(name, func(t *testing.T) {
				extra := ""
				if atExtra {
					extra = "really?"
				}
				h := newTestFlow(t, withExtra("sure?", extra), false)
				planned(h.f)
				pressKeys(h.f, keyEnter)
				if atExtra {
					pressKeys(h.f, keyPress('y'))
				}

				cmd := h.f.Update(k)

				if isRunRequest(cmd) || len(h.ran) != 0 {
					t.Fatalf("%q declined nothing: a run was requested", name)
				}
				if stage := h.f.stage; stage == stageConfirm || stage == stageExtra {
					t.Fatalf("stage = %v after %q, want the confirm closed", stage, name)
				}
			})
		}
	}
}

func TestFlowEscStepsBackOneStagePerStage(t *testing.T) {
	cases := []struct {
		name  string
		st    *fakeStaged
		to    []tea.KeyPressMsg // keys that reach the stage under test, after planning
		stage flowStage
		want  flowStage
	}{
		{"preview to entry", withExtra("c", ""), nil, stagePreview, stageIdle},
		{"confirm to preview", withExtra("c", ""), []tea.KeyPressMsg{keyEnter}, stageConfirm, stagePreview},
		{"offer to preview", &fakeStaged{offer: "Back up? [Y/n]", built: plan.Plan{}}, []tea.KeyPressMsg{keyEnter}, stageOffer, stagePreview},
		{"confirm to offer", &fakeStaged{offer: "Back up? [Y/n]", built: plan.Plan{}}, []tea.KeyPressMsg{keyEnter, keyEnter}, stageConfirm, stageOffer},
		{"extra to confirm", withExtra("c", "e"), []tea.KeyPressMsg{keyEnter, keyPress('y')}, stageExtra, stageConfirm},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newTestFlow(t, c.st, false)
			planned(h.f)
			pressKeys(h.f, c.to...)
			if h.f.stage != c.stage {
				t.Fatalf("setup reached %v, want %v", h.f.stage, c.stage)
			}

			cmd := h.f.Update(keyEsc)

			if h.f.stage != c.want || isRunRequest(cmd) {
				t.Fatalf("esc: stage = %v, want %v and no run", h.f.stage, c.want)
			}
		})
	}
}

func TestFlowSkipPreviewGoesFromPlanningToConfirm(t *testing.T) {
	h := newTestFlow(t, withExtra("wipe?", ""), true)

	planned(h.f)

	if h.f.stage != stageConfirm {
		t.Fatalf("stage = %v, want confirm without a preview", h.f.stage)
	}
	if !isRunRequest(h.f.Update(keyPress('y'))) {
		t.Fatal("y at confirm must produce a runRequest")
	}
}

func TestFlowEnterAtPreviewDoesNothingWhenThePlanCannotApply(t *testing.T) {
	st := withExtra("c", "")
	st.cannotApply = true
	h := newTestFlow(t, st, false)
	planned(h.f)

	cmd := h.f.Update(keyEnter)

	if h.f.stage != stagePreview || isRunRequest(cmd) {
		t.Fatalf("stage = %v, want preview untouched", h.f.stage)
	}
}

func TestFlowYSendsARunRequestWithTheBuiltPlanOtherKeysDoNot(t *testing.T) {
	h := newTestFlow(t, withExtra("sure?", ""), false)
	planned(h.f)

	if isRunRequest(pressKeys(h.f, keyEnter, keyPress('x'))) || len(h.ran) != 0 {
		t.Fatal("a declined confirm ran something")
	}

	planned(h.f)
	req, ok := pressKeys(h.f, keyEnter, keyPress('y'))().(runRequest)

	if !ok || len(h.ran) != 1 || h.ran[0].Title != "built" || req.title != "built" {
		t.Fatalf("got ok=%v ran=%v, want one run of the built plan", ok, h.ran)
	}
}

func TestFlowOfferAnswerReachesBuildAndDefaultsToYes(t *testing.T) {
	answers := map[string]struct {
		k    tea.KeyPressMsg
		want bool
	}{"enter": {keyEnter, true}, "y": {keyPress('y'), true}, "n": {keyPress('n'), false}}
	for name, a := range answers {
		t.Run(name, func(t *testing.T) {
			st := &fakeStaged{offer: "Back up the affected files first? [Y/n]", built: plan.Plan{Title: "built"}}
			h := newTestFlow(t, st, false)
			planned(h.f)

			pressKeys(h.f, keyEnter, a.k)

			if h.f.stage != stageConfirm || len(st.offers) == 0 || st.offers[len(st.offers)-1] != a.want {
				t.Fatalf("stage = %v offers = %v, want confirm and Build(%v)", h.f.stage, st.offers, a.want)
			}
		})
	}
}

func TestPlanStagedListsStepsAndKeepsBothConfirms(t *testing.T) {
	p := plan.Plan{
		Title:        "Do it",
		Steps:        []plan.Step{plan.CommandStep{Label: "run the marker command"}},
		Confirm:      "sure?",
		ExtraConfirm: "really?",
	}
	st := newPlanStaged(p)

	view := strings.Join(st.View(ForVariant(""), 80, 20), "\n")
	label, _ := plan.Describe(p.Steps[0])
	if !strings.Contains(view, label) {
		t.Errorf("view %q lacks the described step %q", view, label)
	}
	if _, offered := st.Offer(); offered {
		t.Error("a bare plan offers a backup")
	}
	if built := st.Build(false); built.Confirm != p.Confirm || built.ExtraConfirm != p.ExtraConfirm {
		t.Errorf("built = %+v, want Confirm and ExtraConfirm kept", built)
	}
	if !st.CanApply() || newPlanStaged(plan.Plan{}).CanApply() {
		t.Error("CanApply must be true for a plan with steps and false for an empty one")
	}
}
