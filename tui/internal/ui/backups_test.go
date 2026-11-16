package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type fakeBackups struct {
	list    []domain.Backup
	deleted []string
	listed  int
	dryRun  bool
}

func (f *fakeBackups) ListBackups(context.Context) ([]domain.Backup, error) {
	f.listed++
	return f.list, nil
}

func (f *fakeBackups) PlanRestore(context.Context, domain.Backup) (app.SyncPlan, error) {
	return app.SyncPlan{Direction: plan.Restore, Items: []app.PlanItem{planItem("a", false)}}, nil
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

func TestBackupsDryRunDeleteShowsWouldDelete(t *testing.T) {
	f := &fakeBackups{list: []domain.Backup{sampleBackup("/b/1")}, dryRun: true}
	b := loadedBackups(f)

	b.Update(keyPress('d'))
	_, cmd := b.Update(keyPress('y'))
	b.Update(cmd())
	if !strings.Contains(ansi.Strip(b.View(100, 20)), "would delete /b/1") {
		t.Errorf("dry-run delete must show the would-delete detail:\n%s", ansi.Strip(b.View(100, 20)))
	}
}

func TestBackupsRefuseIncompleteAndUnreadableRestore(t *testing.T) {
	for _, mark := range []func(*domain.Backup){
		func(bk *domain.Backup) { bk.Incomplete = true },
		func(bk *domain.Backup) { bk.Unreadable = true },
	} {
		bad := sampleBackup("/b/1")
		mark(&bad)
		b := loadedBackups(&fakeBackups{list: []domain.Backup{bad}})

		_, cmd := b.Update(keyPress(tea.KeyEnter))
		if cmd != nil || b.err == nil {
			t.Errorf("restore of %s backup must be refused: cmd=%v err=%v", flag(bad), cmd != nil, b.err)
		}
	}
}

func loadedBackups(f *fakeBackups) *Backups {
	b := newBackups(Deps{Backups: f}, DefaultKeyMap(), NewRoot("", false, Deps{}).palette)
	b.Update(b.load()())
	return b
}

func sampleBackup(dir string) domain.Backup {
	return domain.Backup{Dir: dir, Created: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC), Direction: plan.Pull, Files: []string{"home/a"}}
}

func TestBackupsDeleteDeclineMakesNoCall(t *testing.T) {
	f := &fakeBackups{list: []domain.Backup{sampleBackup("/b/1")}}
	b := loadedBackups(f)

	b.Update(keyPress('d'))
	if !b.Modal() {
		t.Fatal("d must open the confirm dialog")
	}
	_, cmd := b.Update(keyPress('n'))
	if cmd != nil || b.Modal() || len(f.deleted) != 0 {
		t.Errorf("decline must not delete: cmd=%v deleted=%v", cmd != nil, f.deleted)
	}
}

func TestBackupsDeleteConfirmDeletesAndReloads(t *testing.T) {
	f := &fakeBackups{list: []domain.Backup{sampleBackup("/b/1")}}
	b := loadedBackups(f)

	b.Update(keyPress('d'))
	_, cmd := b.Update(keyPress('y'))
	_, reload := b.Update(cmd())
	if len(f.deleted) != 1 || f.deleted[0] != "/b/1" {
		t.Fatalf("deleted = %v", f.deleted)
	}
	before := f.listed
	reload()
	if f.listed != before+1 {
		t.Error("list must reload after delete")
	}
}

func TestBackupsRestoreOpensSyncWithRestorePlan(t *testing.T) {
	f := &fakeBackups{list: []domain.Backup{sampleBackup("/b/1")}}
	r := NewRoot("", false, Deps{Backups: f, Syncer: &fakeSyncer{}})
	r.selected, r.panelFocus = 2, true
	b := r.sections[2].screen.(*Backups)
	b.Update(b.load()())

	_, cmd := b.Update(keyPress(tea.KeyEnter))
	model, _ := r.Update(cmd())
	r = model.(Root)
	s := r.sections[syncSection].screen.(*Sync)
	if r.selected != syncSection || s.phase != phasePreview || s.plan.Direction != plan.Restore {
		t.Errorf("selected=%d phase=%v direction=%v", r.selected, s.phase, s.plan.Direction)
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
	f := &fakeBackups{list: []domain.Backup{bad}}
	b := loadedBackups(f)

	if !strings.Contains(ansi.Strip(b.View(80, 20)), "[incomplete]") {
		t.Error("incomplete backup must be flagged")
	}
	b.Update(keyPress('d'))
	if !b.Modal() {
		t.Error("incomplete backup must be deletable")
	}
}
