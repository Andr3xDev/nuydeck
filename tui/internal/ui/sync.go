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
	case plannedMsg, sidesMsg, editedMsg:
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
	res       cfgsync.Resolutions // edits the owner accepted; only selected items reach the apply
	cursor    int
	confirmed *syncApply

	sides       map[cfgsync.ItemKey]*sideState
	sub         subStage
	subScroll   scroller
	pending     *editedMsg // the edit result waiting for y or n
	editNotices []string
}

func newSyncStaged(svc *services, p cfgsync.SyncPlan, confirmed *syncApply) *syncStaged {
	sel := cfgsync.Selection{}
	maps.Copy(sel, p.Selection)
	return &syncStaged{
		svc: svc, syncPlan: p, sel: sel, res: cfgsync.Resolutions{}, confirmed: confirmed,
		sides: map[cfgsync.ItemKey]*sideState{},
	}
}

func (st *syncStaged) Update(msg tea.KeyPressMsg) tea.Cmd {
	k := st.svc.keys
	if st.sub != subNone {
		return st.updateSub(msg)
	}
	switch {
	case key.Matches(msg, k.Up):
		st.cursor = max(st.cursor-1, 0)
		return st.Load()
	case key.Matches(msg, k.Down):
		st.cursor = max(min(st.cursor+1, len(st.syncPlan.Items)-1), 0)
		return st.Load()
	case key.Matches(msg, k.Toggle):
		st.toggle()
	case key.Matches(msg, k.All):
		st.toggleAll()
	case key.Matches(msg, k.Edit):
		return st.edit()
	case key.Matches(msg, k.Diff):
		st.openDiff()
	}
	return nil
}

// Notices lists the plan's state warning, unless the shell already shows it, its notices and the editor's.
func (st *syncStaged) Notices() []string {
	var out []string
	if w := st.syncPlan.StateWarning; w != "" && w != st.svc.StateWarning {
		out = append(out, "State: "+w)
	}
	return append(append(out, st.syncPlan.Notices...), st.editNotices...)
}

func (st *syncStaged) CanApply() bool { return st.svc.ApplySync != nil && st.selectedCount() > 0 }

func (st *syncStaged) Offer() (string, bool) {
	if st.svc.NeedsBackup == nil || !st.svc.NeedsBackup(st.syncPlan, st.sel) {
		return "", false
	}
	return backupOfferText, true
}

func (st *syncStaged) Build(offer bool) plan.Plan {
	applied := st.syncPlan
	applied.Resolutions = maps.Clone(st.res)
	*st.confirmed = syncApply{plan: applied, sel: maps.Clone(st.sel), backup: offer}
	return plan.Plan{Confirm: fmt.Sprintf("Apply %d items (%s)?", st.selectedCount(), directionLabel(st.syncPlan.Direction))}
}

func (st *syncStaged) current() (cfgsync.PlanItem, bool) {
	if st.cursor >= len(st.syncPlan.Items) {
		return cfgsync.PlanItem{}, false
	}
	return st.syncPlan.Items[st.cursor], true
}

// selectable items offer a choice; extras and blocked items are information only.
func selectable(it cfgsync.PlanItem) bool { return !it.Blocked && it.Change.Type != plan.Delete }

// toggle cycles keep and take; an edited item goes back to keep and loses its edit.
func (st *syncStaged) toggle() {
	it, ok := st.current()
	if !ok || !selectable(it) {
		return
	}
	k := it.Key()
	if _, edited := st.res[k]; edited {
		delete(st.res, k)
		st.sel[k] = false
		return
	}
	st.sel[k] = !st.sel[k]
}

// toggleAll clears the selection (and the edits) when every selectable item is selected, else selects them all.
func (st *syncStaged) toggleAll() {
	all := true
	for _, it := range st.syncPlan.Items {
		all = all && (!selectable(it) || st.sel[it.Key()])
	}
	if all {
		clear(st.res)
	}
	for _, it := range st.syncPlan.Items {
		st.sel[it.Key()] = !all && selectable(it)
	}
}

func (st *syncStaged) selectedCount() int {
	n := 0
	for _, it := range st.syncPlan.Items {
		if st.sel[it.Key()] && selectable(it) {
			n++
		}
	}
	return n
}

func (st *syncStaged) View(th theme, w, h int) []string {
	if st.sub == subDiff {
		return st.diffView(th, w, h)
	}
	header := fmt.Sprintf("%d of %d selected", st.selectedCount(), len(st.syncPlan.Items))
	lines := []string{th.title.Render(truncate(header, w))}
	if len(st.syncPlan.Items) == 0 {
		return append(lines, styled(th.muted, wrap("Nothing to sync.", w))...)
	}
	var popup []string
	if st.sub == subResult {
		popup = st.resultView(th, w, max(h/2, minPopupLines))
	}
	rows := make([]listRow, len(st.syncPlan.Items))
	for i, it := range st.syncPlan.Items {
		rows[i] = st.row(it, w)
	}
	list, at := listLines(th, rows, st.cursor, w)
	lines = append(lines, windowAround(list, at, max(h-len(lines)-len(popup), 1))...)
	return append(lines, popup...)
}

// marker is the row's state: the choice offered, or "extra" for information-only deletes.
func (st *syncStaged) marker(it cfgsync.PlanItem) string {
	k := it.Key()
	_, edited := st.res[k]
	switch {
	case it.Blocked:
		return "[-]"
	case it.Change.Type == plan.Delete:
		return "extra"
	case edited && st.sel[k]:
		return "[e] edit"
	case st.sel[k]:
		return "[x] take"
	case it.Change.Type == plan.Create:
		return "[ ] skip"
	}
	return "[ ] keep"
}

// row is the plan item list row; narrow panels drop the repo and PC times.
func (st *syncStaged) row(it cfgsync.PlanItem, w int) listRow {
	line := fmt.Sprintf("%s %s %s:%s", st.marker(it), changeLabel(it.Change.Type), it.Change.Entry.Root, it.Key().Path)
	if s := st.sides[it.Key()]; s != nil && !s.loaded {
		line += "  loading"
	}
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
