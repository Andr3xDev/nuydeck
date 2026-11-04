package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

type fakeSyncer struct {
	plan        app.SyncPlan
	needsBackup bool
}

func (f *fakeSyncer) PlanSync(context.Context, domain.Direction, []domain.ResolvedEntry) (app.SyncPlan, error) {
	return f.plan, nil
}

func (f *fakeSyncer) NeedsBackup(app.SyncPlan, app.Selection) bool { return f.needsBackup }

func (f *fakeSyncer) Apply(context.Context, app.SyncPlan, app.Selection, app.ApplyOptions, func(domain.StepProgress)) app.ApplyResult {
	return app.ApplyResult{}
}

func planItem(path string, blocked bool) app.PlanItem {
	return app.PlanItem{Change: domain.Change{Entry: domain.Entry{Root: "home", Path: path}, Type: domain.Update}, Blocked: blocked}
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
