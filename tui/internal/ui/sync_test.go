package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

type fakeSyncer struct {
	plan          app.SyncPlan
	needsBackup   bool
	progressPaths []string
	blockUntilCtx bool
	opts          app.ApplyOptions
	applyCalls    int
}

func (f *fakeSyncer) PlanSync(context.Context, domain.Direction, []manifest.ResolvedEntry) (app.SyncPlan, error) {
	return f.plan, nil
}

func (f *fakeSyncer) NeedsBackup(app.SyncPlan, app.Selection) bool { return f.needsBackup }

func (f *fakeSyncer) Apply(ctx context.Context, _ app.SyncPlan, _ app.Selection, opts app.ApplyOptions, progress func(domain.StepProgress)) app.ApplyResult {
	f.applyCalls++
	f.opts = opts
	for _, p := range f.progressPaths {
		progress(domain.StepProgress{Path: p})
	}
	if f.blockUntilCtx {
		<-ctx.Done()
	}
	return app.ApplyResult{}
}

func selectedPlan() app.SyncPlan {
	item := planItem("a", false)
	return app.SyncPlan{Items: []app.PlanItem{item}, Selection: app.Selection{item.Key(): true}}
}

// runCmds feeds every command result back into the screen until no command is left.
func runCmds(s *Sync, cmd tea.Cmd) {
	for cmd != nil {
		_, cmd = s.Update(cmd())
	}
}

func planItem(path string, blocked bool) app.PlanItem {
	return app.PlanItem{Change: domain.Change{Entry: manifest.Entry{Root: "home", Path: path}, Type: domain.Update}, Blocked: blocked}
}

func newPreview(f *fakeSyncer, dryRun bool) *Sync {
	s := newSync(Deps{Syncer: f}, dryRun, DefaultKeyMap(), NewRoot("", false, Deps{}).palette)
	s.Update(planMsg{gen: s.gen, plan: f.plan})
	return s
}

func keyPress(code rune) tea.KeyPressMsg {
	if code == tea.KeySpace {
		return tea.KeyPressMsg{Code: code, Text: " "}
	}
	if code == tea.KeyEnter {
		return tea.KeyPressMsg{Code: code}
	}
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

func TestSyncPreviewShowsPlanNotices(t *testing.T) {
	const notice = "skipped: source missing: home/.gone"
	plan := selectedPlan()
	plan.Notices = []string{notice}
	s := newPreview(&fakeSyncer{plan: plan}, false)

	if !strings.Contains(ansi.Strip(s.View(120, 20)), notice) {
		t.Fatalf("preview does not show %q", notice)
	}
}

func TestSyncToggleRespectsBlockedItems(t *testing.T) {
	plan := app.SyncPlan{Items: []app.PlanItem{planItem("a", false), planItem("b", true)}, Selection: app.Selection{}}
	plan.Selection[plan.Items[0].Key()] = true
	s := newPreview(&fakeSyncer{plan: plan}, false)

	s.Update(keyPress(tea.KeySpace))
	if s.sel[plan.Items[0].Key()] {
		t.Error("space must deselect the selected item")
	}
	s.Update(keyPress('j'))
	s.Update(keyPress(tea.KeySpace))
	if s.sel[plan.Items[1].Key()] {
		t.Error("blocked item must not be selectable")
	}
	s.Update(keyPress('a'))
	if !s.sel[plan.Items[0].Key()] || s.sel[plan.Items[1].Key()] || s.selectedCount() != 1 {
		t.Errorf("a must select only selectable items: %v", s.sel)
	}
	s.Update(keyPress('a'))
	if s.selectedCount() != 0 {
		t.Error("a on a full selection must clear it")
	}
}

func TestSyncEnterDialogsDeclineBackup(t *testing.T) {
	item := planItem("a", false)
	plan := app.SyncPlan{Items: []app.PlanItem{item}, Selection: app.Selection{item.Key(): true}}
	s := newPreview(&fakeSyncer{plan: plan, needsBackup: true}, false)

	s.Update(keyPress(tea.KeyEnter))
	if s.phase != phaseBackupDialog || !s.Modal() {
		t.Fatalf("phase = %v, want backup dialog", s.phase)
	}
	s.Update(keyPress('n'))
	if s.phase != phaseConfirmDialog || s.backup {
		t.Fatalf("phase = %v backup = %v, want confirm dialog without backup", s.phase, s.backup)
	}
	s.Update(keyPress('q'))
	if s.phase != phasePreview {
		t.Error("q must close the dialog")
	}
}

func TestSyncDiscardsStalePlan(t *testing.T) {
	s := newPreview(&fakeSyncer{}, false)
	s.startPlan(domain.Push)
	stale := s.gen
	s.startPlan(domain.Pull)
	s.Update(planMsg{gen: stale, plan: app.SyncPlan{Items: []app.PlanItem{planItem("a", false)}}})
	if s.phase != phasePlanning || len(s.plan.Items) != 0 {
		t.Error("stale plan must be discarded")
	}
}

func TestSyncApplyLoopForwardsOptions(t *testing.T) {
	tests := []struct {
		name       string
		dryRun     bool
		backupKey  rune
		wantBackup bool
	}{
		{"backup accepted in dry-run", true, 'y', true},
		{"backup declined on real run", false, 'n', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeSyncer{plan: selectedPlan(), needsBackup: true, progressPaths: []string{"a", "b"}}
			s := newPreview(f, tt.dryRun)

			s.Update(keyPress(tea.KeyEnter))
			s.Update(keyPress(tt.backupKey))
			_, cmd := s.Update(keyPress('y'))
			if s.phase != phaseApplying {
				t.Fatalf("phase = %v, want applying", s.phase)
			}
			runCmds(s, cmd)

			want := app.ApplyOptions{Backup: tt.wantBackup, DryRun: tt.dryRun}
			if f.opts != want || s.progress != 2 || s.phase != phaseResult {
				t.Errorf("opts=%+v progress=%d phase=%v", f.opts, s.progress, s.phase)
			}
		})
	}
}

func TestSyncNothingSelectedDoesNotApply(t *testing.T) {
	f := &fakeSyncer{plan: app.SyncPlan{Items: []app.PlanItem{planItem("a", false)}}}
	s := newPreview(f, false)

	_, cmd := s.Update(keyPress(tea.KeyEnter))
	if cmd != nil || s.phase != phaseResult || !s.result.Nothing || f.applyCalls != 0 {
		t.Errorf("cmd=%v phase=%v nothing=%v applyCalls=%d", cmd != nil, s.phase, s.result.Nothing, f.applyCalls)
	}
}

func applyingRoot(t *testing.T, f *fakeSyncer) (Root, *Sync, tea.Cmd) {
	t.Helper()
	r := NewRoot("", false, Deps{Syncer: f})
	s := r.sections[syncSection].screen.(*Sync)
	s.Update(planMsg{gen: s.gen, plan: f.plan})
	s.Update(keyPress(tea.KeyEnter))
	_, cmd := s.Update(keyPress('y'))
	if s.phase != phaseApplying {
		t.Fatalf("phase = %v, want applying", s.phase)
	}
	return r, s, cmd
}

func TestRootIgnoresNewPlansWhileApplying(t *testing.T) {
	r, s, _ := applyingRoot(t, &fakeSyncer{plan: selectedPlan()})
	gen := s.gen

	for _, msg := range []tea.Msg{startSyncMsg{direction: domain.Pull}, restorePlanMsg{plan: selectedPlan()}} {
		r.Update(msg)
		if s.phase != phaseApplying || s.gen != gen {
			t.Fatalf("%T changed the running apply: phase=%v gen=%d want gen %d", msg, s.phase, s.gen, gen)
		}
	}
	if !strings.Contains(ansi.Strip(s.View(120, 20)), applyBusyNotice) {
		t.Error("busy notice not shown")
	}
}

func TestRootQuitCancelsRunningApplyWithoutHanging(t *testing.T) {
	r, _, startCmd := applyingRoot(t, &fakeSyncer{plan: selectedPlan(), blockUntilCtx: true, progressPaths: []string{"a"}})
	started := startCmd().(applyStartedMsg)

	press(t, r, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	done := make(chan struct{})
	go func() {
		for range started.ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("apply stream still open after quit")
	}
}

func TestSyncResultView(t *testing.T) {
	s := newPreview(&fakeSyncer{}, true)
	view := func() string { return ansi.Strip(s.View(100, 20)) }

	s.Update(applyDoneMsg{gen: s.gen, result: app.ApplyResult{Nothing: true}})
	if !strings.Contains(view(), "Nothing was done") {
		t.Errorf("missing nothing-done line:\n%s", view())
	}

	s.Update(applyDoneMsg{gen: s.gen, result: app.ApplyResult{
		Outcomes: []app.StepOutcome{{Result: domain.StepResult{Status: domain.StepWouldApply, Detail: "would append log entry"}}},
		Items:    []app.ItemResult{{Key: app.ItemKey{Root: "home", Path: "a"}, Result: app.ItemNotApplied}},
		Err:      errors.New("boom"),
	}})
	for _, want := range []string{"would append log entry", "not applied: home:a", "Failed: boom"} {
		if !strings.Contains(view(), want) {
			t.Errorf("view missing %q:\n%s", want, view())
		}
	}
}
