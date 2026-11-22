package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	updateActionLabel = "Update TUI"
	restartNote       = "Restart nuydeck to run the new version."
)

type fakeUpdater struct {
	ran    []plan.Plan
	runErr error
}

func (f *fakeUpdater) PlanUpdate() plan.Plan {
	return plan.Plan{
		Title:   updateActionLabel,
		Steps:   []plan.Step{plan.CommandStep{Label: "update nuydeck", Argv: []string{"bash", "/repo/update-tui.sh"}}},
		Confirm: "Pull the latest nuydeck and rebuild the TUI?",
	}
}

func (f *fakeUpdater) Run(_ context.Context, p plan.Plan, _ func(plan.Event)) error {
	f.ran = append(f.ran, p)
	return f.runErr
}

// openConfig builds the Config screen as the shell does; the update key is Enter, the screen's one action.
func openConfig(t *testing.T, f *fakeUpdater, withUpdate bool) *Config {
	t.Helper()
	deps := Deps{RunPlan: f.Run}
	if withUpdate {
		deps.Config.PlanUpdate = f.PlanUpdate
	}
	s, _, ok := sectionOf[*Config](NewRoot("", false, deps))
	if !ok {
		t.Fatal("no Config section")
	}
	feed(s, s.Enter())
	return s
}

func TestConfigHidesTheUpdateActionWithoutAPlanUpdate(t *testing.T) {
	f := &fakeUpdater{}

	if view := viewOf(openConfig(t, f, true)); !strings.Contains(view, updateActionLabel) {
		t.Errorf("view lacks the %q action:\n%s", updateActionLabel, view)
	}
	s := openConfig(t, f, false)
	runs := send(s, keyEnter, keyEnter, keyPress('y'))

	if view := viewOf(s); strings.Contains(view, updateActionLabel) || len(runs) != 0 || len(f.ran) != 0 {
		t.Errorf("without PlanUpdate: runs=%d plans run=%d, want no action:\n%s", len(runs), len(f.ran), view)
	}
}

func TestConfigUpdateDeclinedRunsNothing(t *testing.T) {
	f := &fakeUpdater{}
	s := openConfig(t, f, true)

	runs := send(s, keyEnter, keyEnter, keyPress('n'))

	if len(runs) != 0 || len(f.ran) != 0 {
		t.Errorf("runs=%d plans run=%d, want none after declining", len(runs), len(f.ran))
	}
}

func TestConfigUpdateSuccessRunsThePlanAndNotesTheRestart(t *testing.T) {
	f := &fakeUpdater{}
	s := openConfig(t, f, true)

	runs := send(s, keyEnter, keyEnter, keyPress('y'))
	if len(runs) != 1 {
		t.Fatalf("run requests = %d, want 1", len(runs))
	}
	lines, end := runWork(runs[0])

	if end.err != nil || len(f.ran) != 1 {
		t.Fatalf("err=%v plans run=%d, want one successful run", end.err, len(f.ran))
	}
	if !strings.Contains(strings.Join(lines, "\n"), restartNote) {
		t.Errorf("emitted lines %q lack %q", lines, restartNote)
	}
}

func TestConfigUpdateDryRunHasNoRestartNote(t *testing.T) {
	f := &fakeUpdater{}
	s, _, _ := sectionOf[*Config](NewRoot("", true, Deps{RunPlan: f.Run, Config: ConfigDeps{PlanUpdate: f.PlanUpdate}}))

	runs := send(s, keyEnter, keyEnter, keyPress('y'))
	if len(runs) != 1 {
		t.Fatalf("run requests = %d, want 1", len(runs))
	}
	lines, end := runWork(runs[0])

	if end.err != nil || strings.Contains(strings.Join(lines, "\n"), restartNote) {
		t.Errorf("err=%v lines=%q, want a clean run and no restart note: nothing was replaced", end.err, lines)
	}
}

func TestConfigUpdateFailureIsReportedWithoutTheRestartNote(t *testing.T) {
	f := &fakeUpdater{runErr: errors.New("the working tree has uncommitted changes")}
	s := openConfig(t, f, true)

	runs := send(s, keyEnter, keyEnter, keyPress('y'))
	if len(runs) != 1 {
		t.Fatalf("run requests = %d, want 1", len(runs))
	}
	lines, end := runWork(runs[0])

	if !errors.Is(end.err, f.runErr) || strings.Contains(strings.Join(lines, "\n"), restartNote) {
		t.Errorf("err=%v lines=%q, want the script error and no restart note", end.err, lines)
	}
}
