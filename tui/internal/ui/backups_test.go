package ui

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type fakeBackups struct {
	list      []backup.Backup
	ran       []plan.Plan
	restored  []string
	listed    int
	manual    plan.Plan
	manualErr error
}

func (f *fakeBackups) ListBackups(context.Context) ([]backup.Backup, error) {
	f.listed++
	return f.list, nil
}

func (f *fakeBackups) PlanRestore(_ context.Context, b backup.Backup) (cfgsync.SyncPlan, error) {
	f.restored = append(f.restored, b.Dir)
	return cfgsync.SyncPlan{Direction: plan.Restore, Items: []cfgsync.PlanItem{planItem("a", false)}}, nil
}

func (f *fakeBackups) PlanManualBackup(context.Context, []manifest.ResolvedEntry) (plan.Plan, error) {
	return f.manual, f.manualErr
}

func (f *fakeBackups) RunPlan(_ context.Context, p plan.Plan, emit func(plan.Event)) error {
	f.ran = append(f.ran, p)
	emit(plan.Event{Kind: plan.EventLine, Line: "ran " + p.Title})
	return nil
}

func (f *fakeBackups) deps() Deps {
	return Deps{ListBackups: f.ListBackups, PlanRestore: f.PlanRestore, PlanManualBackup: f.PlanManualBackup, RunPlan: f.RunPlan}
}

// deletedDirs is every backup dir the confirmed plans deleted.
func (f *fakeBackups) deletedDirs() []string {
	var dirs []string
	for _, p := range f.ran {
		for _, s := range p.Steps {
			if d, ok := s.(plan.DeleteBackupStep); ok {
				dirs = append(dirs, d.Dir)
			}
		}
	}
	return dirs
}

func sampleBackups(dirs ...string) []backup.Backup {
	list := make([]backup.Backup, len(dirs))
	for i, d := range dirs {
		list[i] = sampleBackup(d)
	}
	return list
}

func TestBackupsDeleteRunsThroughRunPlan(t *testing.T) {
	f := &fakeBackups{list: sampleBackups("/b/1")}
	b := loadedBackups(f)

	runs := send(b, keyPress('d'), keyEnter, keyPress('y'))
	if len(runs) != 1 || len(f.ran) != 0 {
		t.Fatalf("runs = %d ran = %d, want one run request and no work before the run pane starts", len(runs), len(f.ran))
	}
	lines, end := runWork(runs[0])
	if end.err != nil || len(lines) != 1 || !strings.Contains(lines[0], "Delete backups") {
		t.Errorf("work lines = %q end = %v, want the RunPlan events", lines, end.err)
	}
}

func TestBackupsDeletePreviewListsTargetsAndAsksTheCount(t *testing.T) {
	f := &fakeBackups{list: sampleBackups("/b/1", "/b/2")}
	b := loadedBackups(f)

	send(b, keyPress('d'))
	if view := ansi.Strip(b.View(100, 20)); !strings.Contains(view, "/b/1") || strings.Contains(view, "/b/2") {
		t.Errorf("preview must list only the cursor backup:\n%s", view)
	}
	send(b, keyEnter)
	if view := ansi.Strip(b.View(100, 20)); !strings.Contains(view, "Delete 1 backup?") {
		t.Errorf("confirm must ask the count:\n%s", view)
	}
}

func TestBackupsDeleteMarkedRowsWinOverTheCursor(t *testing.T) {
	f := &fakeBackups{list: sampleBackups("/b/1", "/b/2", "/b/3")}
	b := loadedBackups(f)

	send(b, keyPress(tea.KeySpace), keyDown, keyDown, keyPress(tea.KeySpace), keyUp) // mark 1 and 3, cursor on 2
	runs := send(b, keyPress('d'), keyEnter, keyPress('y'))
	for _, r := range runs {
		runWork(r)
	}

	if got := f.deletedDirs(); !slices.Equal(got, []string{"/b/1", "/b/3"}) {
		t.Errorf("deleted = %v, want only the marked backups", got)
	}
}

func TestBackupsRefuseIncompleteAndUnreadableRestore(t *testing.T) {
	for _, mark := range []func(*backup.Backup){
		func(bk *backup.Backup) { bk.Incomplete = true },
		func(bk *backup.Backup) { bk.Unreadable = true },
	} {
		bad := sampleBackup("/b/1")
		mark(&bad)
		b := loadedBackups(&fakeBackups{list: []backup.Backup{bad}})

		_, cmd := b.Update(keyPress(tea.KeyEnter))
		if cmd != nil || b.err == nil {
			t.Errorf("restore of %s backup must be refused: cmd=%v err=%v", flag(bad), cmd != nil, b.err)
		}
	}
}

func loadedBackups(f *fakeBackups) *Backups {
	b := newBackups(testServices(f.deps()))
	b.Update(b.load()())
	return b
}

func sampleBackup(dir string) backup.Backup {
	return backup.Backup{Dir: dir, Created: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC), Direction: plan.Pull, Files: []string{"home/a"}}
}

func TestBackupsDeleteDeclineRunsNothing(t *testing.T) {
	f := &fakeBackups{list: sampleBackups("/b/1")}
	b := loadedBackups(f)

	send(b, keyPress('d'))
	if !claims(b.Keys(), "esc") {
		t.Fatal("d must open the delete flow")
	}
	runs := send(b, keyEnter, keyPress('n'))
	if len(runs) != 0 || b.flow.active() || claims(b.Keys(), "esc") || len(f.ran) != 0 {
		t.Errorf("decline must not delete: runs=%d active=%v ran=%d", len(runs), b.flow.active(), len(f.ran))
	}
}

func TestBackupsDeleteConfirmDeletesAndReloads(t *testing.T) {
	f := &fakeBackups{list: sampleBackups("/b/1")}
	b := loadedBackups(f)

	for _, r := range send(b, keyPress('d'), keyEnter, keyPress('y')) {
		runWork(r)
	}
	if got := f.deletedDirs(); !slices.Equal(got, []string{"/b/1"}) {
		t.Fatalf("deleted = %v", got)
	}
	before := f.listed
	_, reload := b.Update(runClosed{})
	reload()
	if f.listed != before+1 {
		t.Error("list must reload when the run pane closes")
	}
}

func TestBackupsMarking(t *testing.T) {
	b := loadedBackups(&fakeBackups{list: sampleBackups("/b/1", "/b/2")})

	send(b, keyPress(tea.KeySpace))
	if !b.marks["/b/1"] || b.marks["/b/2"] || !strings.Contains(ansi.Strip(b.View(100, 20)), "[x]") {
		t.Fatalf("space must mark the cursor row only: %v", b.marks)
	}
	send(b, keyPress(tea.KeySpace))
	if b.marks["/b/1"] {
		t.Error("space again must unmark")
	}
	send(b, keyPress('a'))
	if !b.marks["/b/1"] || !b.marks["/b/2"] {
		t.Error("a must mark every row")
	}
	send(b, keyPress('a'))
	if b.marks["/b/1"] || b.marks["/b/2"] {
		t.Error("a with every row marked must clear the marks")
	}
}

func TestBackupsMarksDropVanishedBackupsAndClearAfterARun(t *testing.T) {
	f := &fakeBackups{list: sampleBackups("/b/1", "/b/2")}
	b := loadedBackups(f)
	send(b, keyPress('a'))

	f.list = sampleBackups("/b/2")
	b.Update(b.load()())
	if b.marks["/b/1"] || !b.marks["/b/2"] {
		t.Errorf("marks = %v, want only the surviving backup marked", b.marks)
	}
	b.Update(runClosed{})
	if len(b.marks) != 0 {
		t.Errorf("marks = %v, want none after a run", b.marks)
	}
}

func TestBackupsSnapshotPreviewsThenRunsThePlan(t *testing.T) {
	f := &fakeBackups{list: sampleBackups("/b/1"), manual: plan.Plan{Title: "Manual backup", Steps: []plan.Step{plan.BackupStep{Direction: plan.Manual}}}}
	b := loadedBackups(f)

	send(b, keyPress('b'))
	if !b.flow.active() || b.flow.stage != stagePreview || len(f.ran) != 0 {
		t.Fatalf("b must open a preview and write nothing: stage=%v ran=%d", b.flow.stage, len(f.ran))
	}
	runs := send(b, keyEnter, keyPress('y'))
	if len(runs) != 1 {
		t.Fatalf("confirm must start one run, got %d", len(runs))
	}
	runWork(runs[0])
	if len(f.ran) != 1 || f.ran[0].Title != "Manual backup" {
		t.Errorf("ran = %v, want the manual backup plan", f.ran)
	}
}

func TestBackupsSnapshotDeclinedRunsNothing(t *testing.T) {
	f := &fakeBackups{list: sampleBackups("/b/1"), manual: plan.Plan{Steps: []plan.Step{plan.BackupStep{Direction: plan.Manual}}}}
	b := loadedBackups(f)

	if runs := send(b, keyPress('b'), keyEnter, keyPress('n')); len(runs) != 0 || len(f.ran) != 0 {
		t.Errorf("declined snapshot ran: runs=%d ran=%d", len(runs), len(f.ran))
	}
}

func TestBackupsSnapshotWithNothingToBackUpShowsAMessage(t *testing.T) {
	f := &fakeBackups{manualErr: backup.ErrNothingToBackUp}
	b := loadedBackups(f)

	runs := send(b, keyPress('b'), keyEnter)

	if len(runs) != 0 || !strings.Contains(ansi.Strip(b.View(100, 20)), "nothing to back up") || b.flow.canApply() {
		t.Errorf("want an error preview with nothing to run:\n%s", ansi.Strip(b.View(100, 20)))
	}
}

func TestBackupsRestoreKeysOnlyRestoreTheCursorRowAndRDoesNotRefresh(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{keyEnter, keyPress('r')} {
		f := &fakeBackups{list: sampleBackups("/b/1", "/b/2")}
		b := loadedBackups(f)
		send(b, keyPress('a'), keyDown)
		listed := f.listed

		send(b, k)

		if !slices.Equal(f.restored, []string{"/b/2"}) || f.listed != listed {
			t.Errorf("%v: restored = %v listed +%d, want only the cursor row and no refresh", k, f.restored, f.listed-listed)
		}
	}
}

func TestBackupsKeysListJumpAndNoRefresh(t *testing.T) {
	b := loadedBackups(&fakeBackups{list: sampleBackups("/b/1")})

	keys := b.Keys()

	if !claims(keys, "J") || !claims(keys, "K") || !claims(keys, "b") || !claims(keys, "space") || !claims(keys, "a") {
		t.Errorf("keys %v must list jump, snapshot, toggle and all", keys)
	}
	for _, binding := range keys {
		if binding.Help().Desc == "refresh" {
			t.Error("refresh must not be listed")
		}
	}
}

func TestBackupsTableHasHeaderAndColumns(t *testing.T) {
	view := ansi.Strip(loadedBackups(&fakeBackups{list: sampleBackups("/b/1")}).View(100, 20))

	for _, want := range []string{"Date", "Direction", "Files", "Pull", "2026-01-02 03:04"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestBackupsRestoreIsAFlowOwnedByTheScreen(t *testing.T) {
	f := &fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}}
	r := NewRoot("", false, f.deps())
	b, at, _ := sectionOf[*Backups](r)
	r.selected, r.focus = at, focusPanel
	b.Update(b.load()())

	_, cmd := b.Update(keyPress(tea.KeyEnter))
	if !b.flow.active() || b.flow.stage != stagePlanning {
		t.Fatalf("flow active=%v stage=%v, want planning", b.flow.active(), b.flow.stage)
	}
	b.Update(cmd())
	st, ok := b.flow.st.(*syncStaged)
	if r.selected != at || b.flow.stage != stagePreview || !ok || st.syncPlan.Direction != plan.Restore {
		t.Errorf("selected=%d stage=%v preview=%v, want the restore preview on the Backups screen", r.selected, b.flow.stage, ok)
	}
	if !claims(b.Keys(), "esc") {
		t.Error("the restore flow must claim esc")
	}
	b.Update(keyEsc)
	if b.flow.active() || claims(b.Keys(), "esc") {
		t.Error("esc at the preview must leave the flow")
	}
}

func TestBackupsRestorePreviewLoadsRowSides(t *testing.T) {
	f := &fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}}
	deps := f.deps()
	deps.ReadSide = func(string) (cfgsync.Side, error) { return textSide("x"), nil }
	b := newBackups(testServices(deps))
	b.Update(b.load()())

	_, planCmd := b.Update(keyPress(tea.KeyEnter))
	_, loadCmd := b.Update(planCmd())
	if loadCmd == nil {
		t.Fatal("the restore preview issued no side read")
	}
	b.Update(loadCmd())

	st := b.flow.st.(*syncStaged)
	if s := st.sides[itemKey("a")]; s == nil || !s.loaded {
		t.Errorf("row side state = %+v, want loaded", s)
	}
}

func TestBackupsListReloadsAfterAnApplyAndAfterTheRunCloses(t *testing.T) {
	f := &fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}}
	b := loadedBackups(f)

	for _, msg := range []tea.Msg{applyDoneMsg{}, runClosed{}} {
		before := f.listed
		_, reload := b.Update(msg)
		reload()
		if f.listed != before+1 {
			t.Errorf("%T must reload the list", msg)
		}
	}
}

func TestBackupsDropsAListThatArrivesAfterANewerLoad(t *testing.T) {
	b := loadedBackups(&fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}})
	stale := b.load()()
	b.load()

	b.Update(backupsMsg{gen: stale.(backupsMsg).gen})
	if len(b.backups) != 1 {
		t.Error("a list from a superseded load must be ignored")
	}
}

func TestBackupsEmptyState(t *testing.T) {
	b := loadedBackups(&fakeBackups{})
	if !strings.Contains(ansi.Strip(b.View(80, 20)), "No backups yet.") {
		t.Error("missing empty-state message")
	}
}

func TestBackupsIncompleteListedFlaggedAndDeletable(t *testing.T) {
	bad := sampleBackup("/b/1")
	bad.Incomplete = true
	f := &fakeBackups{list: []backup.Backup{bad}}
	b := loadedBackups(f)

	if !strings.Contains(ansi.Strip(b.View(80, 20)), "incomplete") {
		t.Error("incomplete backup must be flagged")
	}
	send(b, keyPress('d'))
	if !claims(b.Keys(), "esc") {
		t.Error("incomplete backup must be deletable")
	}
}
