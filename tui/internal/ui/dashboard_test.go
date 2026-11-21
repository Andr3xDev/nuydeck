package ui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/drift"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

type fakeDrift struct {
	ctxs []context.Context
	out  chan drift.DriftResult
}

func (f *fakeDrift) Drift(ctx context.Context, _ []manifest.ResolvedEntry) <-chan drift.DriftResult {
	f.ctxs = append(f.ctxs, ctx)
	f.out = make(chan drift.DriftResult, 8)
	return f.out
}

func resolved(path string) manifest.ResolvedEntry {
	return manifest.ResolvedEntry{Entry: manifest.Entry{Root: "home", Path: path}}
}

func newTestDashboard(f *fakeDrift, entries ...manifest.ResolvedEntry) *Dashboard {
	root := NewRoot("", false, Deps{Drift: f.Drift, Entries: entries})
	d, _, _ := sectionOf[*Dashboard](root)
	return d
}

func plainView(s screen) string { return ansi.Strip(s.View(100, 20)) }

func TestDashboardIsTheFirstScreen(t *testing.T) {
	root := NewRoot("", false, Deps{})
	var first screen = root.sections[0].screen
	if _, ok := first.(*Dashboard); !ok || first.Title() != "Dashboard" || first.Enter() != nil {
		t.Errorf("first screen = %T %q, want the Dashboard", first, first.Title())
	}
	if !slices.ContainsFunc(first.Keys(), func(b key.Binding) bool { return b.Help().Desc == "refresh" }) {
		t.Error("dashboard keys must offer refresh")
	}
}

func TestDashboardDiscardsStaleGeneration(t *testing.T) {
	f := &fakeDrift{}
	d := newTestDashboard(f, resolved("a"), resolved("b"))
	d.Init()
	staleGen := d.gen

	d.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if d.gen != staleGen+1 {
		t.Fatalf("gen = %d, want %d", d.gen, staleGen+1)
	}
	if f.ctxs[0].Err() == nil {
		t.Error("refresh must cancel the prior context")
	}

	_, cmd := d.Update(driftEntryMsg{gen: staleGen, result: drift.DriftResult{Entry: resolved("a"), State: drift.DriftDrifted, Changes: 3}})
	if cmd != nil || len(d.results) != 0 {
		t.Fatal("stale result must be discarded without re-issuing the read")
	}

	_, cmd = d.Update(driftEntryMsg{gen: d.gen, result: drift.DriftResult{Entry: resolved("a"), State: drift.DriftInSync}})
	if cmd == nil || len(d.results) != 1 {
		t.Fatal("current result must be stored and the read re-issued")
	}
}

func TestDashboardPerEntryStates(t *testing.T) {
	f := &fakeDrift{}
	d := newTestDashboard(f, resolved("a"), resolved("b"), resolved("c"))
	d.Init()
	if strings.Count(plainView(d), "in sync") != 0 || !d.running {
		t.Fatal("entries must show a spinner before results arrive")
	}

	d.Update(driftEntryMsg{gen: d.gen, result: drift.DriftResult{Entry: resolved("a"), State: drift.DriftInSync}})
	d.Update(driftEntryMsg{gen: d.gen, result: drift.DriftResult{Entry: resolved("b"), State: drift.DriftError, Err: errors.New("boom")}})
	view := plainView(d)
	for _, want := range []string{"in sync", "error"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if d.entryStatus(resolved("c").Entry) != d.spinner.View() {
		t.Error("entry without a result must still show the spinner")
	}

	d.Update(driftDoneMsg{gen: d.gen})
	if d.running {
		t.Error("done message must stop the spinner")
	}
}

func TestDashboardMissingSideStatuses(t *testing.T) {
	d := newTestDashboard(&fakeDrift{}, resolved("a"), resolved("b"))
	d.Init()
	d.Update(driftEntryMsg{gen: d.gen, result: drift.DriftResult{Entry: resolved("a"), State: drift.DriftDrifted, Missing: drift.MissingInRepo}})
	d.Update(driftEntryMsg{gen: d.gen, result: drift.DriftResult{Entry: resolved("b"), State: drift.DriftDrifted, Missing: drift.MissingOnPC}})

	view := plainView(d)
	for _, want := range []string{"only in live", "not on PC"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestDashboardMultilineErrorStaysOutOfRowAndShowsOnOneDetailLine(t *testing.T) {
	d := newTestDashboard(&fakeDrift{}, resolved("a"), resolved("b"))
	d.Init()
	d.Update(driftEntryMsg{gen: d.gen, result: drift.DriftResult{Entry: resolved("b"), State: drift.DriftError, Err: errors.New("first line\nsecond line")}})

	d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	var detail []string
	for _, line := range strings.Split(plainView(d), "\n") {
		if strings.Contains(line, "home:b") {
			if !strings.Contains(line, "error") || strings.Contains(line, "first line") {
				t.Errorf("row must hold the short status only: %q", line)
			}
		} else if strings.Contains(line, "first line") {
			detail = append(detail, line)
		}
	}
	if len(detail) != 1 || !strings.Contains(detail[0], "second line") {
		t.Errorf("want the flattened error on exactly one separate line, got %q", detail)
	}

	d.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if strings.Contains(plainView(d), "first line") {
		t.Error("detail line must show only while the cursor is on the error row")
	}
}

func TestDashboardErrorDetailLineKeepsViewWithinHeight(t *testing.T) {
	var entries []manifest.ResolvedEntry
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		entries = append(entries, resolved(p))
	}
	d := newTestDashboard(&fakeDrift{}, entries...)
	d.Init()
	d.Update(driftEntryMsg{gen: d.gen, result: drift.DriftResult{Entry: resolved("a"), State: drift.DriftError, Err: errors.New("boom")}})

	if n := strings.Count(d.View(100, 10), "\n") + 1; n > 10 {
		t.Errorf("view has %d lines with a detail line, want at most 10", n)
	}
}

func TestDashboardRowsFitHeightAndFollowCursor(t *testing.T) {
	var entries []manifest.ResolvedEntry
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t"} {
		entries = append(entries, resolved(p))
	}
	d := newTestDashboard(&fakeDrift{}, entries...)

	if n := strings.Count(d.View(100, 10), "\n") + 1; n > 10 {
		t.Errorf("view has %d lines, want at most 10", n)
	}
	for range entries {
		d.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	if !strings.Contains(plainView(d), "home:t") {
		t.Error("cursor on the last entry must scroll it into view")
	}
}

func TestDashboardRefreshesAfterApply(t *testing.T) {
	f := &fakeDrift{}
	d := newTestDashboard(f, resolved("a"))
	d.Init()
	gen := d.gen

	stamp := &state.LastSync{Direction: "push"}
	_, cmd := d.Update(applyDoneMsg{result: cfgsync.ApplyResult{Outcomes: []cfgsync.StepOutcome{
		{Step: plan.SaveStateStep{State: state.State{LastSync: stamp}}, Result: plan.StepResult{Status: plan.StepOK}},
	}}})
	if cmd == nil || d.gen != gen+1 || len(f.ctxs) != 2 {
		t.Errorf("apply must re-run drift: gen=%d runs=%d", d.gen, len(f.ctxs))
	}
	if d.state.lastSync != stamp {
		t.Error("last sync must adopt the applied state")
	}
}

func TestDashboardPullPushRequestSync(t *testing.T) {
	d := newTestDashboard(&fakeDrift{}, resolved("a"))
	for key, want := range map[rune]plan.Direction{'p': plan.Pull, 'P': plan.Push} {
		_, cmd := d.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
		if got := cmd().(startSyncMsg).direction; got != want {
			t.Errorf("%q: direction = %v, want %v", key, got, want)
		}
	}
}

func TestRootOpensSyncOnStartSync(t *testing.T) {
	r := NewRoot("", false, (&fakeSyncDeps{}).deps())
	_, cmd := r.Update(startSyncMsg{direction: plan.Push})
	sync, at, _ := sectionOf[*Sync](r)
	if r.selected != at || cmd == nil || !sync.flow.active() || sync.flow.stage != stagePlanning || sync.flow.src.Title != directionLabel(plan.Push) {
		t.Errorf("selected=%d cmd=%v flow active=%v, want Sync open at the plan stage for a push", r.selected, cmd != nil, sync.flow.active())
	}
}

func TestRootManifestErrorOffersNoOperations(t *testing.T) {
	r := NewRoot("", false, Deps{ManifestError: errors.New("paths.toml:7: duplicate path")})
	if got := sectionTitles(r); !slices.Equal(got, []string{"Manifest", "Config"}) {
		t.Fatalf("sections = %v, want only the error screen and Config", got)
	}
	r.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	for _, k := range []rune{'p', 'P', 'r'} {
		_, cmd := press(t, r, tea.KeyPressMsg{Code: k, Text: string(k)})
		if cmd != nil {
			t.Errorf("%q must do nothing on manifest error", k)
		}
	}
	if !strings.Contains(ansi.Strip(r.View().Content), "paths.toml:7: duplicate path") {
		t.Error("manifest error not shown")
	}
	r.selectSection(1)
	if !strings.Contains(ansi.Strip(r.View().Content), "Repo path:") {
		t.Error("Config must stay usable and show its values on manifest error")
	}
}
