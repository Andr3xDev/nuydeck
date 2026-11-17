package ui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type backupsMsg struct {
	gen     int
	backups []backup.Backup
	err     error
}

// Backups lists existing backups and owns the restore flow and the delete confirm.
type Backups struct {
	svc          *services
	listBackups  func(ctx context.Context) ([]backup.Backup, error)
	planRestore  func(ctx context.Context, b backup.Backup) (cfgsync.SyncPlan, error)
	deleteBackup func(ctx context.Context, dir string, confirmed bool) (plan.StepResult, bool)
	keys         keyMap
	th           theme
	gen          int // drops a list that arrives after a newer load
	backups      []backup.Backup
	cursor       int
	err          error
	confirm      bool
	flow         *flow
}

func newBackups(svc *services) *Backups {
	return &Backups{
		svc:          svc,
		listBackups:  svc.ListBackups,
		planRestore:  svc.PlanRestore,
		deleteBackup: svc.DeleteBackup,
		keys:         svc.keys,
		th:           svc.th,
	}
}

func (b *Backups) Title() string { return "Backups" }

// Init loads the list at startup; Enter reloads it.
func (b *Backups) Init() tea.Cmd  { return b.load() }
func (b *Backups) Enter() tea.Cmd { return b.load() }

// Keys claims esc while the restore flow or the delete dialog is open, so esc steps back instead of leaving the panel.
func (b *Backups) Keys() []key.Binding {
	if b.flow.active() {
		return b.flow.Keys()
	}
	if b.confirm {
		return []key.Binding{b.keys.Yes, b.keys.No, b.keys.Back}
	}
	keys := []key.Binding{b.keys.Up, b.keys.Down, b.keys.Refresh}
	if _, ok := b.selected(); ok && b.wired() {
		keys = append(keys, b.keys.Delete, b.keys.Confirm)
	}
	return keys
}

// wired is false when restore and delete are not available.
func (b *Backups) wired() bool { return b.planRestore != nil && b.deleteBackup != nil }

func (b *Backups) load() tea.Cmd {
	if b.listBackups == nil {
		return nil
	}
	b.gen++
	gen, listBackups := b.gen, b.listBackups
	return func() tea.Msg {
		list, err := listBackups(context.Background())
		return backupsMsg{gen: gen, backups: list, err: err}
	}
}

func (b *Backups) selected() (backup.Backup, bool) {
	if b.cursor >= len(b.backups) {
		return backup.Backup{}, false
	}
	return b.backups[b.cursor], true
}

func (b *Backups) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case backupsMsg:
		if msg.gen == b.gen {
			b.backups, b.err = msg.backups, msg.err
			b.cursor = max(min(b.cursor, len(b.backups)-1), 0)
		}
		return b, nil
	case applyDoneMsg, runClosed:
		if b.flow.active() {
			b.flow.reset()
		}
		return b, b.load()
	case plannedMsg:
		if b.flow.active() {
			return b, b.flow.Update(msg)
		}
	case tea.KeyPressMsg:
		return b.handleKey(msg)
	}
	return b, nil
}

func (b *Backups) handleKey(msg tea.KeyPressMsg) (screen, tea.Cmd) {
	if b.flow.active() {
		return b, b.flow.Update(msg)
	}
	if b.confirm {
		switch {
		case key.Matches(msg, b.keys.Yes):
			b.confirm = false
			return b, b.delete()
		case key.Matches(msg, b.keys.No, b.keys.Back):
			b.confirm = false
		}
		return b, nil
	}
	sel, ok := b.selected()
	switch {
	case key.Matches(msg, b.keys.Refresh):
		return b, b.load()
	case key.Matches(msg, b.keys.Up):
		b.cursor = max(b.cursor-1, 0)
	case key.Matches(msg, b.keys.Down):
		b.cursor = max(min(b.cursor+1, len(b.backups)-1), 0)
	case !ok || !b.wired():
	case key.Matches(msg, b.keys.Delete):
		b.confirm = true
	case key.Matches(msg, b.keys.Confirm):
		return b, b.restore(sel)
	}
	return b, nil
}

func (b *Backups) restore(sel backup.Backup) tea.Cmd {
	if err := backup.CheckRestorable(sel); err != nil {
		b.err = fmt.Errorf("backup %s cannot be restored: %w", sel.Dir, err)
		return nil
	}
	b.err = nil
	planRestore := b.planRestore
	b.flow = newFlow(b.svc, syncSource(b.svc, directionLabel(plan.Restore), func(ctx context.Context) (cfgsync.SyncPlan, error) {
		return planRestore(ctx, sel)
	}))
	return b.flow.start()
}

// delete asks the run to remove the selected backup; a refusal or failure ends the run with an error.
func (b *Backups) delete() tea.Cmd {
	sel, ok := b.selected()
	if !ok {
		return nil
	}
	deleteBackup := b.deleteBackup
	req := runRequest{title: "Delete backup", work: func(ctx context.Context, emit func(plan.Event)) runEnd {
		res, _ := deleteBackup(ctx, sel.Dir, true)
		switch res.Status {
		case plan.StepFailed:
			return runEnd{err: fmt.Errorf("delete failed: %s", res.Detail)}
		case plan.StepWouldApply:
			emit(plan.Event{Kind: plan.EventLine, Line: res.Detail})
		}
		return runEnd{}
	}}
	return func() tea.Msg { return req }
}

func (b *Backups) View(width, height int) string {
	if b.flow.active() {
		return b.flow.box(b.Title(), width, height)
	}
	cw := contentWidth(width)
	inner := innerHeight(height)
	var lines []string
	if b.err != nil {
		lines = append(lines, styled(b.th.err, wrap(b.err.Error(), cw))...)
	}
	if len(b.backups) == 0 {
		lines = append(lines, b.th.muted.Render("No backups yet."))
		return screenBox(b.th, "Backups", lines, width, height)
	}
	lines = append(lines, b.th.title.Render(fmt.Sprintf("%d backups, newest first", len(b.backups))))
	rows := make([]listRow, len(b.backups))
	for i, bk := range b.backups {
		rows[i] = b.row(bk, width)
	}
	list, at := listLines(b.th, rows, b.cursor, cw)
	lines = append(lines, windowAround(list, at, max((inner-len(lines))/2, 1))...)
	sel := b.backups[b.cursor]
	lines = append(lines, "", b.th.title.Render(truncate(sanitizeLine(sel.Dir), cw)))
	if b.confirm {
		lines = append(lines, styled(b.th.warning, wrap(fmt.Sprintf("Delete this backup (%d files)? (y/n)", len(sel.Files)), cw))...)
	}
	room := max(inner-len(lines), 1)
	files := make([]string, 0, min(len(sel.Files), room))
	for _, f := range sel.Files {
		if len(files) == room-1 && len(sel.Files) > room {
			files = append(files, fmt.Sprintf("... %d more", len(sel.Files)-room+1))
			break
		}
		files = append(files, truncate(sanitizeLine(f), cw))
	}
	return screenBox(b.th, "Backups", append(lines, files...), width, height)
}

// row is the backup list row; narrow panels drop direction and file count.
func (b *Backups) row(bk backup.Backup, width int) listRow {
	row := listRow{text: bk.Created.Format(timeLayout)}
	if width >= wideRowWidth {
		row.text += fmt.Sprintf("  %s %d files", directionLabel(bk.Direction), len(bk.Files))
	}
	if f := flag(bk); f != "" {
		row.badges = []string{f}
	}
	return row
}

func flag(bk backup.Backup) string {
	switch {
	case bk.Unreadable:
		return "unreadable"
	case bk.Incomplete:
		return "incomplete"
	}
	return ""
}
