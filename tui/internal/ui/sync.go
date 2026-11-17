package ui

import (
	"cmp"
	"context"
	"fmt"
	"maps"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	backupOfferText = "Back up the affected files first? [Y/n]"
	nothingDoneText = "Nothing was done."
)

// applyDoneMsg is broadcast when a sync or restore apply ends, so screens can refresh what it changed.
type applyDoneMsg struct{ result cfgsync.ApplyResult }

// Sync starts a push or a pull and hosts its flow.
type Sync struct {
	svc  *services
	flow *flow
}

func newSync(svc *services) *Sync { return &Sync{svc: svc} }

func (s *Sync) Title() string  { return "Sync" }
func (s *Sync) Enter() tea.Cmd { return nil }

func (s *Sync) Keys() []key.Binding {
	if s.flow.active() {
		return s.flow.Keys()
	}
	return []key.Binding{s.svc.keys.Pull, s.svc.keys.Push}
}

// startPlan always recomputes: a plan is never cached (spec).
func (s *Sync) startPlan(dir plan.Direction) tea.Cmd {
	if s.svc.PlanSync == nil {
		return nil
	}
	entries := s.svc.Entries
	s.flow = newFlow(s.svc, syncSource(s.svc, directionLabel(dir), func(ctx context.Context) (cfgsync.SyncPlan, error) {
		return s.svc.PlanSync(ctx, dir, entries)
	}))
	return s.flow.start()
}

func (s *Sync) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case runClosed, applyDoneMsg:
		if s.flow.active() {
			s.flow.reset()
		}
	case plannedMsg:
		if s.flow.active() {
			return s, s.flow.Update(msg)
		}
	case tea.KeyPressMsg:
		keys := s.svc.keys
		switch {
		case s.flow.active():
			return s, s.flow.Update(msg)
		case key.Matches(msg, keys.Pull):
			return s, s.startPlan(plan.Pull)
		case key.Matches(msg, keys.Push):
			return s, s.startPlan(plan.Push)
		}
	}
	return s, nil
}

func (s *Sync) Notices() []string {
	if s.flow == nil {
		return nil
	}
	return s.flow.Notices()
}

func (s *Sync) View(width, height int) string {
	if s.flow.active() {
		return s.flow.box(s.Title(), width, height)
	}
	hint := styled(s.svc.th.muted, wrap("Press p to pull or P to push.", contentWidth(width)))
	return screenBox(s.svc.th, s.Title(), hint, width, height)
}

// syncApply is what the owner confirmed: the plan, the selection and the answer to the backup offer.
type syncApply struct {
	plan   cfgsync.SyncPlan
	sel    cfgsync.Selection
	backup bool
}

// request runs the confirmed apply through Deps.ApplySync; the run ends with the result broadcast as applyDoneMsg.
func (a syncApply) request(svc *services, title string) runRequest {
	opts := cfgsync.ApplyOptions{Backup: a.backup, DryRun: svc.dryRun}
	return runRequest{title: title, work: func(ctx context.Context, emit func(plan.Event)) runEnd {
		line := func(text string) {
			if text != "" {
				emit(plan.Event{Kind: plan.EventLine, Line: text})
			}
		}
		res := svc.ApplySync(ctx, a.plan, a.sel, opts, func(p plan.StepProgress) { line(cmp.Or(p.Line, p.Path)) })
		if res.Nothing {
			line(nothingDoneText)
		}
		for _, text := range resultLines(res) {
			line(text)
		}
		return runEnd{err: res.Err, msg: applyDoneMsg{res}}
	}}
}

// syncSource is the flow of a push, a pull and a restore: each previews a cfgsync.SyncPlan and applies it through Deps.ApplySync.
func syncSource(svc *services, title string, planFn func(context.Context) (cfgsync.SyncPlan, error)) flowSource {
	var confirmed syncApply
	return flowSource{
		Title: title,
		Plan: func(ctx context.Context, _ []string) (staged, error) {
			p, err := planFn(ctx)
			if err != nil {
				return nil, err
			}
			return newSyncStaged(svc, p, &confirmed), nil
		},
		Run: func(plan.Plan) runRequest { return confirmed.request(svc, title) },
	}
}

// syncStaged is the preview of a SyncPlan with a selection per item.
type syncStaged struct {
	svc       *services
	syncPlan  cfgsync.SyncPlan
	sel       cfgsync.Selection
	cursor    int
	confirmed *syncApply
}

func newSyncStaged(svc *services, p cfgsync.SyncPlan, confirmed *syncApply) *syncStaged {
	sel := cfgsync.Selection{}
	maps.Copy(sel, p.Selection)
	return &syncStaged{svc: svc, syncPlan: p, sel: sel, confirmed: confirmed}
}

func (st *syncStaged) Keys() []key.Binding {
	k := st.svc.keys
	return []key.Binding{k.Up, k.Down, k.Toggle, k.All}
}

func (st *syncStaged) Update(msg tea.KeyPressMsg) tea.Cmd {
	k := st.svc.keys
	switch {
	case key.Matches(msg, k.Up):
		st.cursor = max(st.cursor-1, 0)
	case key.Matches(msg, k.Down):
		st.cursor = max(min(st.cursor+1, len(st.syncPlan.Items)-1), 0)
	case key.Matches(msg, k.Toggle):
		st.toggle()
	case key.Matches(msg, k.All):
		st.toggleAll()
	}
	return nil
}

// Notices lists the plan's state warning, unless the shell already shows it, and its notices.
func (st *syncStaged) Notices() []string {
	var out []string
	if w := st.syncPlan.StateWarning; w != "" && w != st.svc.StateWarning {
		out = append(out, "State: "+w)
	}
	return append(out, st.syncPlan.Notices...)
}

func (st *syncStaged) CanApply() bool { return st.svc.ApplySync != nil && st.selectedCount() > 0 }

func (st *syncStaged) Offer() (string, bool) {
	if st.svc.NeedsBackup == nil || !st.svc.NeedsBackup(st.syncPlan, st.sel) {
		return "", false
	}
	return backupOfferText, true
}

func (st *syncStaged) Build(offer bool) plan.Plan {
	*st.confirmed = syncApply{plan: st.syncPlan, sel: maps.Clone(st.sel), backup: offer}
	return plan.Plan{Confirm: fmt.Sprintf("Apply %d items (%s)?", st.selectedCount(), directionLabel(st.syncPlan.Direction))}
}

func (st *syncStaged) toggle() {
	if st.cursor >= len(st.syncPlan.Items) {
		return
	}
	if it := st.syncPlan.Items[st.cursor]; !it.Blocked {
		st.sel[it.Key()] = !st.sel[it.Key()]
	}
}

// toggleAll clears the selection when every selectable item is selected, else selects them all.
func (st *syncStaged) toggleAll() {
	all := true
	for _, it := range st.syncPlan.Items {
		all = all && (it.Blocked || st.sel[it.Key()])
	}
	for _, it := range st.syncPlan.Items {
		st.sel[it.Key()] = !all && !it.Blocked
	}
}

func (st *syncStaged) selectedCount() int {
	n := 0
	for _, it := range st.syncPlan.Items {
		if st.sel[it.Key()] && !it.Blocked {
			n++
		}
	}
	return n
}

func (st *syncStaged) View(th theme, w, h int) []string {
	header := fmt.Sprintf("%d of %d selected", st.selectedCount(), len(st.syncPlan.Items))
	lines := []string{th.title.Render(truncate(header, w))}
	if len(st.syncPlan.Items) == 0 {
		return append(lines, styled(th.muted, wrap("Nothing to sync.", w))...)
	}
	rows := make([]listRow, len(st.syncPlan.Items))
	for i, it := range st.syncPlan.Items {
		rows[i] = st.row(it, w)
	}
	list, at := listLines(th, rows, st.cursor, w)
	return append(lines, windowAround(list, at, max(h-len(lines), 1))...)
}

// row is the plan item list row; narrow panels drop the repo and PC times.
func (st *syncStaged) row(it cfgsync.PlanItem, w int) listRow {
	box := "[ ]"
	switch {
	case it.Blocked:
		box = "[-]"
	case st.sel[it.Key()]:
		box = "[x]"
	}
	line := fmt.Sprintf("%s %s %s:%s", box, changeLabel(it.Change.Type), it.Change.Entry.Root, it.Key().Path)
	if w >= wideRowWidth {
		line += fmt.Sprintf("  repo %s  pc %s", timeText(it.RepoTime), timeText(it.PCTime))
	}
	return listRow{text: line, badges: badges(it), muted: it.Blocked}
}

func resultLines(res cfgsync.ApplyResult) []string {
	var lines []string
	for _, o := range res.Outcomes {
		if o.Result.Status == plan.StepWouldApply {
			lines = append(lines, o.Result.Detail)
		}
	}
	for _, it := range res.Items {
		lines = append(lines, fmt.Sprintf("%s: %s:%s", it.Result, it.Key.Root, it.Key.Path))
	}
	return lines
}
