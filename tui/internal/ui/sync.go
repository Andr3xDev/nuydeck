package ui

import (
	"context"
	"fmt"
	"maps"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type planMsg struct {
	gen  int
	plan cfgsync.SyncPlan
	err  error
}

// applyStartedMsg hands the progress channel to the screen (D9).
type applyStartedMsg struct {
	gen int
	ch  <-chan progressEvent
}

type progressMsg struct {
	gen  int
	path string
}

type applyDoneMsg struct {
	gen    int
	result cfgsync.ApplyResult
}

// progressEvent is either one progress path or, last, the final result.
type progressEvent struct {
	path string
	done *cfgsync.ApplyResult
}

type syncPhase int

const (
	phaseIdle syncPhase = iota
	phasePlanning
	phasePreview
	phaseBackupDialog
	phaseConfirmDialog
	phaseApplying
	phaseResult
)

// Sync is the Plan -> Preview -> Apply screen.
type Sync struct {
	ctx         context.Context
	planSync    func(ctx context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (cfgsync.SyncPlan, error)
	needsBackup func(p cfgsync.SyncPlan, sel cfgsync.Selection) bool
	applySync   func(ctx context.Context, p cfgsync.SyncPlan, sel cfgsync.Selection, opts cfgsync.ApplyOptions, progress func(plan.StepProgress)) cfgsync.ApplyResult
	entries     []manifest.ResolvedEntry
	dryRun      bool
	keys        keyMap
	th          theme

	gen       int
	phase     syncPhase
	direction plan.Direction
	plan      cfgsync.SyncPlan
	sel       cfgsync.Selection
	cursor    int
	err       error
	backup    bool
	stream    <-chan progressEvent
	progress  int
	result    cfgsync.ApplyResult
	notice    string
	cancel    context.CancelFunc
}

const applyBusyNotice = "Another sync was ignored: an apply is in progress."

func newSync(svc *services) *Sync {
	return &Sync{
		ctx:         svc.ctx,
		planSync:    svc.PlanSync,
		needsBackup: svc.NeedsBackup,
		applySync:   svc.ApplySync,
		entries:     svc.Entries,
		dryRun:      svc.dryRun,
		keys:        svc.keys,
		th:          svc.th,
	}
}

func (s *Sync) Title() string  { return "Sync" }
func (s *Sync) Enter() tea.Cmd { return nil }

// Keys claims esc while a dialog or the apply is open, so esc reaches the screen instead of leaving the panel.
func (s *Sync) Keys() []key.Binding {
	switch s.phase {
	case phaseBackupDialog, phaseConfirmDialog:
		return []key.Binding{s.keys.Yes, s.keys.No, s.keys.Back}
	case phaseApplying:
		return []key.Binding{s.keys.Back}
	}
	return []key.Binding{s.keys.Up, s.keys.Down, s.keys.Toggle, s.keys.All, s.keys.Confirm, s.keys.Pull, s.keys.Push}
}

// Cancel stops the running plan or apply, if any.
func (s *Sync) Cancel() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// busy reports a running apply and leaves a notice: a new plan must not replace it.
func (s *Sync) busy() bool {
	if s.phase != phaseApplying {
		return false
	}
	s.notice = applyBusyNotice
	return true
}

// startPlan always recomputes: a plan is never cached (spec).
func (s *Sync) startPlan(dir plan.Direction) tea.Cmd {
	if s.busy() {
		return nil
	}
	s.Cancel()
	s.gen++
	s.direction, s.err = dir, nil
	if s.planSync == nil {
		s.phase = phaseIdle
		return nil
	}
	s.phase = phasePlanning
	ctx, cancel := context.WithCancel(s.ctx)
	s.cancel = cancel
	gen, planSync, entries := s.gen, s.planSync, s.entries
	return func() tea.Msg {
		plan, err := planSync(ctx, dir, entries)
		return planMsg{gen: gen, plan: plan, err: err}
	}
}

// showPlan opens the preview on a plan computed elsewhere (restore).
func (s *Sync) showPlan(plan cfgsync.SyncPlan) {
	if s.busy() {
		return
	}
	s.Cancel()
	s.gen++
	s.direction, s.err = plan.Direction, nil
	s.loadPlan(plan)
}

func (s *Sync) loadPlan(plan cfgsync.SyncPlan) {
	s.plan, s.cursor, s.phase = plan, 0, phasePreview
	s.sel = cfgsync.Selection{}
	maps.Copy(s.sel, plan.Selection)
}

func (s *Sync) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case planMsg:
		if msg.gen != s.gen {
			return s, nil
		}
		if msg.err != nil {
			s.err, s.phase = msg.err, phaseResult
			s.result = cfgsync.ApplyResult{}
			return s, nil
		}
		s.loadPlan(msg.plan)
		return s, nil
	case applyStartedMsg:
		if msg.gen != s.gen {
			return s, nil
		}
		s.stream = msg.ch
		return s, waitProgress(msg.gen, msg.ch)
	case progressMsg:
		if msg.gen != s.gen {
			return s, nil
		}
		s.progress++
		return s, waitProgress(msg.gen, s.stream)
	case applyDoneMsg:
		if msg.gen != s.gen {
			return s, nil
		}
		s.result, s.phase = msg.result, phaseResult
		return s, nil
	case tea.KeyPressMsg:
		return s.handleKey(msg)
	}
	return s, nil
}

func (s *Sync) handleKey(msg tea.KeyPressMsg) (screen, tea.Cmd) {
	switch s.phase {
	case phaseApplying:
		return s, nil
	case phaseBackupDialog:
		switch {
		case key.Matches(msg, s.keys.Yes), key.Matches(msg, s.keys.No):
			s.backup = key.Matches(msg, s.keys.Yes)
			s.phase = phaseConfirmDialog
		case key.Matches(msg, s.keys.Back):
			s.phase = phasePreview
		}
		return s, nil
	case phaseConfirmDialog:
		switch {
		case key.Matches(msg, s.keys.Yes):
			return s, s.startApply()
		case key.Matches(msg, s.keys.No, s.keys.Back):
			s.phase = phasePreview
		}
		return s, nil
	}

	switch {
	case key.Matches(msg, s.keys.Pull):
		return s, s.startPlan(plan.Pull)
	case key.Matches(msg, s.keys.Push):
		return s, s.startPlan(plan.Push)
	case s.phase != phasePreview:
		return s, nil
	case key.Matches(msg, s.keys.Up):
		s.cursor = max(s.cursor-1, 0)
	case key.Matches(msg, s.keys.Down):
		s.cursor = min(s.cursor+1, len(s.plan.Items)-1)
	case key.Matches(msg, s.keys.Toggle):
		s.toggle()
	case key.Matches(msg, s.keys.All):
		s.toggleAll()
	case key.Matches(msg, s.keys.Confirm):
		return s, s.confirm()
	}
	return s, nil
}

func (s *Sync) toggle() {
	if s.cursor >= len(s.plan.Items) {
		return
	}
	if it := s.plan.Items[s.cursor]; !it.Blocked {
		s.sel[it.Key()] = !s.sel[it.Key()]
	}
}

// toggleAll clears the selection when every selectable item is selected, else selects them all.
func (s *Sync) toggleAll() {
	all := true
	for _, it := range s.plan.Items {
		all = all && (it.Blocked || s.sel[it.Key()])
	}
	for _, it := range s.plan.Items {
		s.sel[it.Key()] = !all && !it.Blocked
	}
}

func (s *Sync) selectedCount() int {
	n := 0
	for _, it := range s.plan.Items {
		if s.sel[it.Key()] && !it.Blocked {
			n++
		}
	}
	return n
}

// confirm walks the dialogs; with nothing selected there is nothing to confirm.
func (s *Sync) confirm() tea.Cmd {
	switch {
	case s.selectedCount() == 0:
		s.err, s.result, s.phase = nil, cfgsync.ApplyResult{Nothing: true}, phaseResult
		return nil
	case s.needsBackup != nil && s.needsBackup(s.plan, s.sel):
		s.phase = phaseBackupDialog
	default:
		s.backup = false
		s.phase = phaseConfirmDialog
	}
	return nil
}

func (s *Sync) startApply() tea.Cmd {
	s.Cancel()
	s.gen++
	s.phase, s.progress, s.notice = phaseApplying, 0, ""
	ctx, cancel := context.WithCancel(s.ctx)
	s.cancel = cancel
	gen, applySync, syncPlan, opts := s.gen, s.applySync, s.plan, cfgsync.ApplyOptions{Backup: s.backup, DryRun: s.dryRun}
	sel := maps.Clone(s.sel)
	return func() tea.Msg {
		ch := make(chan progressEvent)
		// send gives up once ctx is cancelled, so a consumer that stopped reading cannot hang Apply.
		send := func(ev progressEvent) {
			select {
			case ch <- ev:
			case <-ctx.Done():
			}
		}
		go func() {
			defer close(ch)
			defer cancel()
			res := applySync(ctx, syncPlan, sel, opts, func(p plan.StepProgress) {
				send(progressEvent{path: p.Path})
			})
			send(progressEvent{done: &res})
		}()
		return applyStartedMsg{gen: gen, ch: ch}
	}
}

// waitProgress reads one event and is re-issued by Update until the final result (D9).
func waitProgress(gen int, ch <-chan progressEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok || ev.done != nil {
			var res cfgsync.ApplyResult
			if ev.done != nil {
				res = *ev.done
			}
			return applyDoneMsg{gen: gen, result: res}
		}
		return progressMsg{gen: gen, path: ev.path}
	}
}

func (s *Sync) View(width, height int) string {
	return screenBox(s.th, s.Title(), s.body(width, innerHeight(height)), width, height)
}

func (s *Sync) body(width, height int) []string {
	cw := contentWidth(width)
	switch s.phase {
	case phaseIdle:
		return s.muted("Press p to pull or P to push.", cw)
	case phasePlanning:
		return s.muted("Planning "+directionLabel(s.direction)+"...", cw)
	case phaseApplying:
		quit := s.keys.Quit.Help()
		lines := wrap(fmt.Sprintf("Applying... %d items transferred (%s to %s and interrupt the run)", s.progress, quit.Key, quit.Desc), cw)
		if s.notice != "" {
			lines = append(append(lines, ""), s.warn(s.notice, cw)...)
		}
		return lines
	case phaseResult:
		return s.resultView(cw, height)
	}
	var dialog []string
	switch s.phase {
	case phaseBackupDialog:
		dialog = s.warn("Back up files that will be overwritten or deleted first? (y/n)", cw)
	case phaseConfirmDialog:
		dialog = s.warn(fmt.Sprintf("Apply %d items (%s)? (y/n)", s.selectedCount(), directionLabel(s.direction)), cw)
	}
	if dialog == nil {
		return s.previewView(width, height)
	}
	return append(append(s.previewView(width, max(height-len(dialog)-1, 1)), ""), dialog...)
}

func (s *Sync) previewView(width, height int) []string {
	cw := contentWidth(width)
	header := fmt.Sprintf("%s: %d of %d selected", directionLabel(s.direction), s.selectedCount(), len(s.plan.Items))
	lines := []string{s.th.title.Render(truncate(header, cw))}
	if s.plan.StateWarning != "" {
		lines = append(lines, s.warn("State: "+s.plan.StateWarning, cw)...)
	}
	for _, notice := range s.plan.Notices {
		lines = append(lines, s.warn(notice, cw)...)
	}
	if len(s.plan.Items) == 0 {
		return append(lines, s.muted("Nothing to sync.", cw)...)
	}
	rows := make([]listRow, len(s.plan.Items))
	for i, it := range s.plan.Items {
		rows[i] = s.row(it, width)
	}
	list, at := listLines(s.th, rows, s.cursor, cw)
	return append(lines, windowAround(list, at, max(height-len(lines), 1))...)
}

// row is the plan item list row; narrow panels drop the repo and PC times.
func (s *Sync) row(it cfgsync.PlanItem, width int) listRow {
	box := "[ ]"
	switch {
	case it.Blocked:
		box = "[-]"
	case s.sel[it.Key()]:
		box = "[x]"
	}
	line := fmt.Sprintf("%s %s %s:%s", box, changeLabel(it.Change.Type), it.Change.Entry.Root, it.Key().Path)
	if width >= wideRowWidth {
		line += fmt.Sprintf("  repo %s  pc %s", timeText(it.RepoTime), timeText(it.PCTime))
	}
	return listRow{text: line, badges: badges(it), muted: it.Blocked}
}

func (s *Sync) resultView(cw, height int) []string {
	var lines []string
	switch {
	case s.err != nil:
		lines = styled(s.th.err, wrap("Plan failed: "+s.err.Error(), cw))
	case s.result.Nothing:
		lines = []string{"Nothing was done."}
	default:
		for _, l := range resultLines(s.result) {
			lines = append(lines, wrap(l, cw)...)
		}
		if s.result.Err != nil {
			lines = append(lines, styled(s.th.err, wrap("Failed: "+s.result.Err.Error(), cw))...)
		}
	}
	if len(lines) > height {
		lines = append(lines[:height-1], s.muted(fmt.Sprintf("... %d more lines", len(lines)-height+1), cw)...)
	}
	return lines
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

func (s *Sync) muted(text string, cw int) []string {
	return styled(s.th.muted, wrap(text, cw))
}
func (s *Sync) warn(text string, cw int) []string { return styled(s.th.warning, wrap(text, cw)) }
