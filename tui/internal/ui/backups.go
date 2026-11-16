package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/theme"
)

// BackupManager is the backup use-case surface the screen needs; *app.Service implements it.
type BackupManager interface {
	ListBackups(ctx context.Context) ([]domain.Backup, error)
	PlanRestore(ctx context.Context, b domain.Backup) (app.SyncPlan, error)
	DeleteBackup(ctx context.Context, dir string, confirmed bool) (plan.StepResult, bool)
}

type backupsMsg struct {
	backups []domain.Backup
	err     error
}

// deleteDoneMsg carries a failure or, in dry-run, the "would delete" detail.
type deleteDoneMsg struct {
	err    error
	notice string
}

// restorePlanMsg is intercepted by Root, which opens the Sync preview on success.
type restorePlanMsg struct {
	plan app.SyncPlan
	err  error
}

// Backups lists existing backups and starts restore and delete.
type Backups struct {
	manager BackupManager
	keys    KeyMap
	palette theme.Palette
	backups []domain.Backup
	cursor  int
	err     error
	confirm bool
	notice  string
}

func newBackups(deps Deps, keys KeyMap, palette theme.Palette) *Backups {
	return &Backups{manager: deps.Backups, keys: keys, palette: palette}
}

// Init loads the list at startup; enter reloads it.
func (b *Backups) Init() tea.Cmd { return b.load() }

func (b *Backups) enter() tea.Cmd { return b.load() }

// Modal reports whether q and esc must close the delete dialog instead of reaching the shell.
func (b *Backups) Modal() bool { return b.confirm }

func (b *Backups) load() tea.Cmd {
	if b.manager == nil {
		return nil
	}
	manager := b.manager
	return func() tea.Msg {
		list, err := manager.ListBackups(context.Background())
		return backupsMsg{backups: list, err: err}
	}
}

func (b *Backups) selected() (domain.Backup, bool) {
	if b.cursor >= len(b.backups) {
		return domain.Backup{}, false
	}
	return b.backups[b.cursor], true
}

func (b *Backups) Update(msg tea.Msg) (Screen, tea.Cmd) {
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

func (b *Backups) handleKey(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
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
	case !ok || b.manager == nil:
	case key.Matches(msg, b.keys.Delete):
		b.confirm = true
	case key.Matches(msg, b.keys.Confirm):
		return b, b.restore(sel)
	}
	return b, nil
}

func (b *Backups) restore(sel domain.Backup) tea.Cmd {
	if sel.Incomplete || sel.Unreadable {
		b.err = fmt.Errorf("backup %s is %s and cannot be restored", sel.Dir, flag(sel))
		return nil
	}
	b.err = nil
	manager := b.manager
	return func() tea.Msg {
		plan, err := manager.PlanRestore(context.Background(), sel)
		return restorePlanMsg{plan: plan, err: err}
	}
}

func (b *Backups) delete() tea.Cmd {
	sel, ok := b.selected()
	if !ok {
		return nil
	}
	manager := b.manager
	return func() tea.Msg {
		res, _ := manager.DeleteBackup(context.Background(), sel.Dir, true)
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
	return renderPanel(width, height, b.body(width, max(height-2*panelPadY, 1)))
}

func (b *Backups) body(width, height int) string {
	var lines []string
	if b.err != nil {
		lines = append(lines, b.colored(b.palette.Error, wrapToPanel(width, b.err.Error())))
	}
	if b.notice != "" {
		lines = append(lines, b.colored(b.palette.Warning, b.notice))
	}
	if len(b.backups) == 0 {
		return strings.Join(append(lines, b.colored(b.palette.TextMuted, "No backups yet.")), "\n")
	}
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d backups, newest first", len(b.backups))))
	listHeight := max((height-len(lines))/2, 1)
	start := max(min(b.cursor-listHeight/2, len(b.backups)-listHeight), 0)
	for i := start; i < min(start+listHeight, len(b.backups)); i++ {
		lines = append(lines, b.row(i, b.backups[i]))
	}
	sel := b.backups[b.cursor]
	lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render(sel.Dir))
	if b.confirm {
		lines = append(lines, b.colored(b.palette.Warning, fmt.Sprintf("Delete this backup (%d files)? (y/n)", len(sel.Files))))
	}
	room := max(height-len(lines), 1)
	files := sel.Files
	if len(files) > room {
		files = append(files[:room-1:room-1], fmt.Sprintf("... %d more", len(sel.Files)-room+1))
	}
	return strings.Join(append(lines, files...), "\n")
}

func (b *Backups) row(i int, bk domain.Backup) string {
	cursor := "  "
	if i == b.cursor {
		cursor = "> "
	}
	line := fmt.Sprintf("%s%s  %-7s %d files", cursor, bk.Created.Format(timeLayout), directionLabel(bk.Direction), len(bk.Files))
	if f := flag(bk); f != "" {
		line += " " + b.colored(b.palette.Warning, "["+f+"]")
	}
	return line
}

func flag(bk domain.Backup) string {
	switch {
	case bk.Unreadable:
		return "unreadable"
	case bk.Incomplete:
		return "incomplete"
	}
	return ""
}

func (b *Backups) colored(c color.Color, text string) string {
	return lipgloss.NewStyle().Foreground(c).Render(text)
}
