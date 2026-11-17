package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type fakeBackups struct {
	list    []backup.Backup
	deleted []string
	listed  int
	dryRun  bool
}

func (f *fakeBackups) ListBackups(context.Context) ([]backup.Backup, error) {
	f.listed++
	return f.list, nil
}

func (f *fakeBackups) PlanRestore(context.Context, backup.Backup) (cfgsync.SyncPlan, error) {
	return cfgsync.SyncPlan{Direction: plan.Restore, Items: []cfgsync.PlanItem{planItem("a", false)}}, nil
}

func (f *fakeBackups) DeleteBackup(_ context.Context, dir string, confirmed bool) (plan.StepResult, bool) {
	if !confirmed {
		return plan.StepResult{}, false
	}
	f.deleted = append(f.deleted, dir)
	if f.dryRun {
		return plan.StepResult{Status: plan.StepWouldApply, Detail: "would delete " + dir}, true
	}
	return plan.StepResult{}, true
}

func (f *fakeBackups) deps() Deps {
	return Deps{ListBackups: f.ListBackups, PlanRestore: f.PlanRestore, DeleteBackup: f.DeleteBackup}
}

func TestBackupsDryRunDeleteShowsWouldDelete(t *testing.T) {
	f := &fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}, dryRun: true}
	b := loadedBackups(f)

	b.Update(keyPress('d'))
	_, cmd := b.Update(keyPress('y'))
	lines, end := runWork(cmd().(runRequest))
	if len(lines) != 1 || lines[0] != "would delete /b/1" || end.err != nil {
		t.Errorf("dry-run delete lines = %q end = %v, want the would-delete detail", lines, end.err)
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

func TestBackupsDeleteDeclineMakesNoCall(t *testing.T) {
	f := &fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}}
	b := loadedBackups(f)

	b.Update(keyPress('d'))
	if !claims(b.Keys(), "esc") {
		t.Fatal("d must open the confirm dialog")
	}
	_, cmd := b.Update(keyPress('n'))
	if cmd != nil || claims(b.Keys(), "esc") || len(f.deleted) != 0 {
		t.Errorf("decline must not delete: cmd=%v deleted=%v", cmd != nil, f.deleted)
	}
}

func TestBackupsDeleteConfirmDeletesAndReloads(t *testing.T) {
	f := &fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}}
	b := loadedBackups(f)

	b.Update(keyPress('d'))
	_, cmd := b.Update(keyPress('y'))
	if _, end := runWork(cmd().(runRequest)); end.err != nil {
		t.Fatalf("delete failed: %v", end.err)
	}
	if len(f.deleted) != 1 || f.deleted[0] != "/b/1" {
		t.Fatalf("deleted = %v", f.deleted)
	}
	before := f.listed
	_, reload := b.Update(runClosed{})
	reload()
	if f.listed != before+1 {
		t.Error("list must reload when the run pane closes")
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

	if !strings.Contains(ansi.Strip(b.View(80, 20)), "[incomplete]") {
		t.Error("incomplete backup must be flagged")
	}
	b.Update(keyPress('d'))
	if !claims(b.Keys(), "esc") {
		t.Error("incomplete backup must be deletable")
	}
}
