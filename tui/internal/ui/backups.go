package ui

import (
	"context"
	"fmt"
	"slices"
	"strconv"

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

// Backups lists existing backups and owns the restore, delete and manual backup flows.
type Backups struct {
	svc         *shell
	listBackups func(ctx context.Context) ([]backup.Backup, error)
	planRestore func(ctx context.Context, b backup.Backup) (cfgsync.SyncPlan, error)
	keys        keyMap
	th          theme
	gen         int // drops a list that arrives after a newer load
	backups     []backup.Backup
	marks       map[string]bool // keyed by Backup.Dir
	cursor      int
	err         error
	flow        *flow
}

func newBackups(svc *shell) *Backups {
	return &Backups{
		svc:         svc,
		listBackups: svc.ListBackups,
		planRestore: svc.PlanRestore,
		keys:        svc.keys,
		th:          svc.th,
		marks:       map[string]bool{},
	}
}

func (b *Backups) Title() string { return "Backups" }

// Init loads the list at startup; Enter reloads it.
func (b *Backups) Init() tea.Cmd  { return b.load() }
func (b *Backups) Enter() tea.Cmd { return b.load() }

// Keys claims esc while a flow is open, so esc steps back instead of leaving the panel.
func (b *Backups) Keys() []key.Binding {
	if b.flow.active() {
		return b.flow.Keys()
	}
	keys := b.keys.move()
	if len(b.backups) > 0 {
		keys = append(keys, b.keys.Toggle, b.keys.All)
		if b.svc.RunPlan != nil {
			keys = append(keys, b.keys.Delete)
		}
		if b.planRestore != nil {
			keys = append(keys, b.keys.Restore)
		}
	}
	if b.canSnapshot() {
		keys = append(keys, b.keys.Snapshot)
	}
	return keys
}

func (b *Backups) canSnapshot() bool { return b.svc.PlanManualBackup != nil && b.svc.RunPlan != nil }

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
			for dir := range b.marks {
				if !slices.ContainsFunc(b.backups, func(bk backup.Backup) bool { return bk.Dir == dir }) {
					delete(b.marks, dir)
				}
			}
		}
		return b, nil
	case applyDoneMsg, runClosed:
		if b.flow.active() {
			b.flow.reset()
		}
		clear(b.marks)
		return b, b.load()
	case plannedMsg, sidesMsg, editedMsg:
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
	if delta := b.keys.step(msg); delta != 0 {
		b.cursor = moveCursor(b.cursor, delta, len(b.backups))
		return b, nil
	}
	sel, ok := b.selected()
	switch {
	case key.Matches(msg, b.keys.Snapshot) && b.canSnapshot():
		return b, b.snapshot()
	case !ok:
	case key.Matches(msg, b.keys.Toggle):
		b.marks[sel.Dir] = !b.marks[sel.Dir]
	case key.Matches(msg, b.keys.All):
		b.markAll()
	case key.Matches(msg, b.keys.Delete) && b.svc.RunPlan != nil:
		return b, b.delete(sel)
	case key.Matches(msg, b.keys.Restore) && b.planRestore != nil:
		return b, b.restore(sel)
	}
	return b, nil
}

// markAll marks every row, or clears every mark when all are already marked.
func (b *Backups) markAll() {
	all := !slices.ContainsFunc(b.backups, func(bk backup.Backup) bool { return !b.marks[bk.Dir] })
	for _, bk := range b.backups {
		b.marks[bk.Dir] = !all
	}
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

// delete removes the marked backups, or the cursor row when none is marked, after the flow's confirm.
func (b *Backups) delete(sel backup.Backup) tea.Cmd {
	targets := []backup.Backup{sel}
	if marked := slices.DeleteFunc(slices.Clone(b.backups), func(bk backup.Backup) bool { return !b.marks[bk.Dir] }); len(marked) > 0 {
		targets = marked
	}
	p := plan.Plan{Title: "Delete backups", Confirm: fmt.Sprintf("Delete %d backups?", len(targets))}
	for _, t := range targets {
		p.Steps = append(p.Steps, plan.DeleteBackupStep{Dir: t.Dir})
	}
	b.err = nil
	return b.startPlanFlow(p.Title, func(context.Context) (plan.Plan, error) { return p, nil })
}

// snapshot previews a manual backup of every manifest entry that exists on the PC.
func (b *Backups) snapshot() tea.Cmd {
	entries, planManual := b.svc.Entries, b.svc.PlanManualBackup
	b.err = nil
	return b.startPlanFlow("Manual backup", func(ctx context.Context) (plan.Plan, error) {
		return planManual(ctx, entries)
	})
}

func (b *Backups) startPlanFlow(title string, planFn func(context.Context) (plan.Plan, error)) tea.Cmd {
	b.flow = newFlow(b.svc, flowSource{
		Title: title,
		Plan: func(ctx context.Context, _ []string) (staged, error) {
			p, err := planFn(ctx)
			if err != nil {
				return nil, err
			}
			return newPlanStaged(p), nil
		},
		Run: func(p plan.Plan) runRequest { return planRunRequest(b.svc, p) },
	})
	return b.flow.start()
}

// planRunRequest runs a confirmed plan through Deps.RunPlan in the run pane.
func planRunRequest(svc *shell, p plan.Plan) runRequest {
	steps := make([]string, len(p.Steps))
	for i, step := range p.Steps {
		steps[i], _ = plan.Describe(step)
	}
	return runRequest{title: p.Title, steps: steps, work: func(ctx context.Context, emit func(plan.Event)) runEnd {
		return runEnd{err: svc.RunPlan(ctx, p, emit)}
	}}
}

var backupColumns = []column{
	{title: "Date"},
	{title: "Direction", priority: 3},
	{title: "Files", priority: 2},
	{title: "Flag", priority: 1},
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
	rows := make([]tableRow, len(b.backups))
	for i, bk := range b.backups {
		rows[i] = tableRow{cells: []string{
			checkbox(b.marks[bk.Dir]) + " " + bk.Created.Format(timeLayout),
			directionLabel(bk.Direction),
			strconv.Itoa(len(bk.Files)),
			flag(bk),
		}}
	}
	header, list, at := tableLines(b.th, backupColumns, rows, b.cursor, cw)
	lines = append(lines, header)
	lines = append(lines, windowAround(list, at, max((inner-len(lines))/2, 1))...)
	sel := b.backups[b.cursor]
	lines = append(lines, "", b.th.title.Render(truncate(sanitizeLine(sel.Dir), cw)))
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

func flag(bk backup.Backup) string {
	switch {
	case bk.Unreadable:
		return "unreadable"
	case bk.Incomplete:
		return "incomplete"
	}
	return ""
}
