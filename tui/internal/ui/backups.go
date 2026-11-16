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
	backups []backup.Backup
	err     error
}

// deleteDoneMsg carries a failure or, in dry-run, the "would delete" detail.
type deleteDoneMsg struct {
	err    error
	notice string
}

// restorePlanMsg is intercepted by Root, which opens the Sync preview on success.
type restorePlanMsg struct {
	plan cfgsync.SyncPlan
	err  error
}

// Backups lists existing backups and starts restore and delete.
type Backups struct {
	listBackups  func(ctx context.Context) ([]backup.Backup, error)
	planRestore  func(ctx context.Context, b backup.Backup) (cfgsync.SyncPlan, error)
	deleteBackup func(ctx context.Context, dir string, confirmed bool) (plan.StepResult, bool)
	keys         KeyMap
	th           theme
	backups      []backup.Backup
	cursor       int
	err          error
	confirm      bool
	notice       string
}

func newBackups(svc *services) *Backups {
	return &Backups{
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

// Keys claims esc while the delete dialog is open, so esc and q close it instead of reaching the shell.
func (b *Backups) Keys() []key.Binding {
	if b.confirm {
		return []key.Binding{b.keys.Yes, b.keys.No, b.keys.Back}
	}
	return []key.Binding{b.keys.Up, b.keys.Down, b.keys.Refresh, b.keys.Delete, b.keys.Confirm}
}

// wired is false when restore and delete are not available.
func (b *Backups) wired() bool { return b.planRestore != nil && b.deleteBackup != nil }

func (b *Backups) load() tea.Cmd {
	if b.listBackups == nil {
		return nil
	}
	listBackups := b.listBackups
	return func() tea.Msg {
		list, err := listBackups(context.Background())
		return backupsMsg{backups: list, err: err}
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
		b.backups, b.err = msg.backups, msg.err
		b.cursor = max(min(b.cursor, len(b.backups)-1), 0)
		return b, nil
	case deleteDoneMsg:
		b.err, b.notice = msg.err, msg.notice
		return b, b.load()
	case restorePlanMsg:
		b.err = msg.err
		return b, nil
	case tea.KeyPressMsg:
		return b.handleKey(msg)
	}
	return b, nil
}

func (b *Backups) handleKey(msg tea.KeyPressMsg) (screen, tea.Cmd) {
	if b.confirm {
		switch {
		case key.Matches(msg, b.keys.Yes):
			b.confirm = false
			return b, b.delete()
		case key.Matches(msg, b.keys.No), key.Matches(msg, b.keys.Back), key.Matches(msg, b.keys.Quit):
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
	return func() tea.Msg {
		plan, err := planRestore(context.Background(), sel)
		return restorePlanMsg{plan: plan, err: err}
	}
}

func (b *Backups) delete() tea.Cmd {
	sel, ok := b.selected()
	if !ok {
		return nil
	}
	deleteBackup := b.deleteBackup
	return func() tea.Msg {
		res, _ := deleteBackup(context.Background(), sel.Dir, true)
		switch res.Status {
		case plan.StepFailed:
			return deleteDoneMsg{err: fmt.Errorf("delete failed: %s", res.Detail)}
		case plan.StepWouldApply:
			return deleteDoneMsg{notice: res.Detail}
		}
		return deleteDoneMsg{}
	}
}

func (b *Backups) View(width, height int) string {
	cw := contentWidth(width)
	inner := innerHeight(height)
	var lines []string
	if b.err != nil {
		lines = append(lines, styled(b.th.err, wrap(b.err.Error(), cw))...)
	}
	if b.notice != "" {
		lines = append(lines, styled(b.th.warning, wrap(b.notice, cw))...)
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
