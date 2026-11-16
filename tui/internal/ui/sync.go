package ui

import (
	"context"
	"fmt"
	"maps"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Syncer is the sync use-case surface the screen needs; *cfgsync.Service implements it.
type Syncer interface {
	PlanSync(ctx context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (cfgsync.SyncPlan, error)
	NeedsBackup(plan cfgsync.SyncPlan, sel cfgsync.Selection) bool
	Apply(ctx context.Context, plan cfgsync.SyncPlan, sel cfgsync.Selection, opts cfgsync.ApplyOptions, progress func(plan.StepProgress)) cfgsync.ApplyResult
}

type planMsg struct {
	gen  int
	plan cfgsync.SyncPlan
	err  error
}

// applyStartedMsg hands the progress channel to the screen (D9).
type applyStartedMsg struct {
	gen int
	ch  <-chan applyEvent
}

type progressMsg struct {
	gen  int
	path string
}

type applyDoneMsg struct {
	gen    int
	result cfgsync.ApplyResult
}

// applyEvent is either one progress path or, last, the final result.
type applyEvent struct {
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
	syncer  Syncer
	entries []manifest.ResolvedEntry
	dryRun  bool
	keys    KeyMap
	palette Palette

	gen       int
	phase     syncPhase
	direction plan.Direction
	plan      cfgsync.SyncPlan
	sel       cfgsync.Selection
	cursor    int
	err       error
	backup    bool
	stream    <-chan applyEvent
	progress  int
	result    cfgsync.ApplyResult
	notice    string
	cancel    context.CancelFunc
}

const applyBusyNotice = "Another sync was ignored: an apply is in progress."

func newSync(deps Deps, dryRun bool, keys KeyMap, palette Palette) *Sync {
	return &Sync{syncer: deps.Syncer, entries: deps.Entries, dryRun: dryRun, keys: keys, palette: palette}
}

// Modal reports whether q and esc must reach the screen instead of the shell.
func (s *Sync) Modal() bool {
	return s.phase == phaseBackupDialog || s.phase == phaseConfirmDialog || s.phase == phaseApplying
}

// cancelWork stops the running plan or apply, if any.
func (s *Sync) cancelWork() {
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
	s.cancelWork()
	s.gen++
	s.direction, s.err = dir, nil
	if s.syncer == nil {
		s.phase = phaseIdle
		return nil
	}
	s.phase = phasePlanning
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	gen, syncer, entries := s.gen, s.syncer, s.entries
	return func() tea.Msg {
		plan, err := syncer.PlanSync(ctx, dir, entries)
		return planMsg{gen: gen, plan: plan, err: err}
	}
}

// showPlan opens the preview on a plan computed elsewhere (restore).
func (s *Sync) showPlan(plan cfgsync.SyncPlan) {
	if s.busy() {
		return
	}
	s.cancelWork()
	s.gen++
	s.direction, s.err = plan.Direction, nil
	s.loadPlan(plan)
}

func (s *Sync) loadPlan(plan cfgsync.SyncPlan) {
	s.plan, s.cursor, s.phase = plan, 0, phasePreview
	s.sel = cfgsync.Selection{}
	maps.Copy(s.sel, plan.Selection)
}

func (s *Sync) Update(msg tea.Msg) (Screen, tea.Cmd) {
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

func (s *Sync) handleKey(msg tea.KeyPressMsg) (Screen, tea.Cmd) {
	switch s.phase {
	case phaseApplying:
		return s, nil
	case phaseBackupDialog:
		switch {
		case key.Matches(msg, s.keys.Yes), key.Matches(msg, s.keys.No):
			s.backup = key.Matches(msg, s.keys.Yes)
			s.phase = phaseConfirmDialog
		case s.dismiss(msg):
			s.phase = phasePreview
		}
		return s, nil
	case phaseConfirmDialog:
		switch {
		case key.Matches(msg, s.keys.Yes):
			return s, s.startApply()
		case key.Matches(msg, s.keys.No), s.dismiss(msg):
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

func (s *Sync) dismiss(msg tea.KeyPressMsg) bool {
	return key.Matches(msg, s.keys.Back) || key.Matches(msg, s.keys.Quit)
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
	case s.syncer.NeedsBackup(s.plan, s.sel):
		s.phase = phaseBackupDialog
	default:
		s.backup = false
		s.phase = phaseConfirmDialog
	}
	return nil
}

func (s *Sync) startApply() tea.Cmd {
	s.cancelWork()
	s.gen++
	s.phase, s.progress, s.notice = phaseApplying, 0, ""
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	gen, syncer, syncPlan, opts := s.gen, s.syncer, s.plan, cfgsync.ApplyOptions{Backup: s.backup, DryRun: s.dryRun}
	sel := maps.Clone(s.sel)
	return func() tea.Msg {
		ch := make(chan applyEvent)
		// send gives up once ctx is cancelled, so a consumer that stopped reading cannot hang Apply.
		send := func(ev applyEvent) {
			select {
			case ch <- ev:
			case <-ctx.Done():
			}
		}
		go func() {
			defer close(ch)
			defer cancel()
			res := syncer.Apply(ctx, syncPlan, sel, opts, func(p plan.StepProgress) {
				send(applyEvent{path: p.Path})
			})
			send(applyEvent{done: &res})
		}()
		return applyStartedMsg{gen: gen, ch: ch}
	}
}

// waitProgress reads one event and is re-issued by Update until the final result (D9).
func waitProgress(gen int, ch <-chan applyEvent) tea.Cmd {
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
	return screenBox(s.palette, "Sync", s.body(width, innerHeight(height)), width, height)
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
	lines := []string{lipgloss.NewStyle().Bold(true).Render(truncate(header, cw))}
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
		rows[i] = listRow{text: s.itemText(it, width), muted: it.Blocked}
	}
	list, at := listLines(newTheme(s.palette), rows, s.cursor, cw)
	return append(lines, windowAround(list, at, max(height-len(lines), 1))...)
}

// itemText is the plain row text; narrow panels drop the repo and PC times.
func (s *Sync) itemText(it cfgsync.PlanItem, width int) string {
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
	for _, badge := range badges(it) {
		line += " [" + badge + "]"
	}
	return line
}

func (s *Sync) resultView(cw, height int) []string {
	var lines []string
	switch {
	case s.err != nil:
		lines = styled(s.palette.Error, wrap("Plan failed: "+s.err.Error(), cw))
	case s.result.Nothing:
		lines = []string{"Nothing was done."}
	default:
		for _, l := range resultLines(s.result) {
			lines = append(lines, wrap(l, cw)...)
		}
		if s.result.Err != nil {
			lines = append(lines, styled(s.palette.Error, wrap("Failed: "+s.result.Err.Error(), cw))...)
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
	return styled(s.palette.TextMuted, wrap(text, cw))
}
func (s *Sync) warn(text string, cw int) []string { return styled(s.palette.Warning, wrap(text, cw)) }
