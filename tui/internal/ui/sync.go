package ui

import (
	"context"
	"fmt"
	"image/color"
	"maps"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/theme"
)

// Syncer is the sync use-case surface the screen needs; *app.Service implements it.
type Syncer interface {
	PlanSync(ctx context.Context, dir domain.Direction, entries []domain.ResolvedEntry) (app.SyncPlan, error)
	NeedsBackup(plan app.SyncPlan, sel app.Selection) bool
	Apply(ctx context.Context, plan app.SyncPlan, sel app.Selection, opts app.ApplyOptions, progress func(domain.StepProgress)) app.ApplyResult
}

type planMsg struct {
	gen  int
	plan app.SyncPlan
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
	result app.ApplyResult
}

// applyEvent is either one progress path or, last, the final result.
type applyEvent struct {
	path string
	done *app.ApplyResult
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
	entries []domain.ResolvedEntry
	dryRun  bool
	keys    KeyMap
	palette theme.Palette

	gen       int
	phase     syncPhase
	direction domain.Direction
	plan      app.SyncPlan
	sel       app.Selection
	cursor    int
	err       error
	backup    bool
	stream    <-chan applyEvent
	progress  int
	result    app.ApplyResult
}

func newSync(deps Deps, dryRun bool, keys KeyMap, palette theme.Palette) *Sync {
	return &Sync{syncer: deps.Syncer, entries: deps.Entries, dryRun: dryRun, keys: keys, palette: palette}
}

// Modal reports whether q and esc must reach the screen instead of the shell.
func (s *Sync) Modal() bool {
	return s.phase == phaseBackupDialog || s.phase == phaseConfirmDialog || s.phase == phaseApplying
}

// startPlan always recomputes: a plan is never cached (spec).
func (s *Sync) startPlan(dir domain.Direction) tea.Cmd {
	s.gen++
	s.direction, s.err = dir, nil
	if s.syncer == nil {
		s.phase = phaseIdle
		return nil
	}
	s.phase = phasePlanning
	gen, syncer, entries := s.gen, s.syncer, s.entries
	return func() tea.Msg {
		plan, err := syncer.PlanSync(context.Background(), dir, entries)
		return planMsg{gen: gen, plan: plan, err: err}
	}
}

// showPlan opens the preview on a plan computed elsewhere (restore).
func (s *Sync) showPlan(plan app.SyncPlan) {
	s.gen++
	s.direction, s.err = plan.Direction, nil
	s.loadPlan(plan)
}

func (s *Sync) loadPlan(plan app.SyncPlan) {
	s.plan, s.cursor, s.phase = plan, 0, phasePreview
	s.sel = app.Selection{}
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
			s.result = app.ApplyResult{}
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
		return s, s.startPlan(domain.Pull)
	case key.Matches(msg, s.keys.Push):
		return s, s.startPlan(domain.Push)
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
		s.backup = false
		return s.startApply()
	case s.syncer.NeedsBackup(s.plan, s.sel):
		s.phase = phaseBackupDialog
	default:
		s.backup = false
		s.phase = phaseConfirmDialog
	}
	return nil
}

func (s *Sync) startApply() tea.Cmd {
	s.gen++
	s.phase, s.progress = phaseApplying, 0
	gen, syncer, plan, opts := s.gen, s.syncer, s.plan, app.ApplyOptions{Backup: s.backup, DryRun: s.dryRun}
	sel := maps.Clone(s.sel)
	return func() tea.Msg {
		ch := make(chan applyEvent)
		go func() {
			defer close(ch)
			res := syncer.Apply(context.Background(), plan, sel, opts, func(p domain.StepProgress) {
				ch <- applyEvent{path: p.Path}
			})
			ch <- applyEvent{done: &res}
		}()
		return applyStartedMsg{gen: gen, ch: ch}
	}
}

// waitProgress reads one event and is re-issued by Update until the final result (D9).
func waitProgress(gen int, ch <-chan applyEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok || ev.done != nil {
			var res app.ApplyResult
			if ev.done != nil {
				res = *ev.done
			}
			return applyDoneMsg{gen: gen, result: res}
		}
		return progressMsg{gen: gen, path: ev.path}
	}
}

func (s *Sync) View(width, height int) string {
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(s.body(max(height-2, 1)))
}

func (s *Sync) body(height int) string {
	switch s.phase {
	case phaseIdle:
		return s.muted("Press p to pull or P to push.")
	case phasePlanning:
		return s.muted("Planning " + directionLabel(s.direction) + "...")
	case phaseApplying:
		return fmt.Sprintf("Applying... %d items transferred", s.progress)
	case phaseResult:
		return s.resultView(height)
	}
	preview := s.previewView(height - 2)
	switch s.phase {
	case phaseBackupDialog:
		return preview + "\n\n" + s.warn("Back up files that will be overwritten or deleted first? (y/n)")
	case phaseConfirmDialog:
		return preview + "\n\n" + s.warn(fmt.Sprintf("Apply %d items (%s)? (y/n)", s.selectedCount(), directionLabel(s.direction)))
	}
	return preview
}

func (s *Sync) previewView(height int) string {
	header := fmt.Sprintf("%s: %d of %d selected", directionLabel(s.direction), s.selectedCount(), len(s.plan.Items))
	lines := []string{lipgloss.NewStyle().Bold(true).Render(header)}
	if s.plan.StateWarning != "" {
		lines = append(lines, s.warn("State: "+s.plan.StateWarning))
	}
	if len(s.plan.Items) == 0 {
		return strings.Join(append(lines, s.muted("Nothing to sync.")), "\n")
	}
	visible := max(height-len(lines), 1)
	start := max(min(s.cursor-visible/2, len(s.plan.Items)-visible), 0)
	for i := start; i < min(start+visible, len(s.plan.Items)); i++ {
		lines = append(lines, s.itemLine(i, s.plan.Items[i]))
	}
	return strings.Join(lines, "\n")
}

func (s *Sync) itemLine(i int, it app.PlanItem) string {
	box := "[ ]"
	switch {
	case it.Blocked:
		box = "[-]"
	case s.sel[it.Key()]:
		box = "[x]"
	}
	cursor := "  "
	if i == s.cursor {
		cursor = "> "
	}
	line := fmt.Sprintf("%s%s %-6s %s:%s  repo %s  pc %s", cursor, box, changeLabel(it.Change.Type),
		it.Change.Entry.Root, it.Key().Path, timeText(it.RepoTime), timeText(it.PCTime))
	for _, badge := range badges(it) {
		line += " " + s.warn("["+badge+"]")
	}
	return line
}

func (s *Sync) resultView(height int) string {
	var lines []string
	switch {
	case s.err != nil:
		lines = append(lines, s.colored(s.palette.Error, "Plan failed: "+s.err.Error()))
	case s.result.Nothing:
		lines = append(lines, "Nothing was done.")
	default:
		lines = append(lines, resultLines(s.result)...)
		if s.result.Err != nil {
			lines = append(lines, s.colored(s.palette.Error, "Failed: "+s.result.Err.Error()))
		}
	}
	if len(lines) > height {
		lines = append(lines[:height-1], s.muted(fmt.Sprintf("... %d more lines", len(lines)-height+1)))
	}
	return strings.Join(lines, "\n")
}

func resultLines(res app.ApplyResult) []string {
	var lines []string
	for _, o := range res.Outcomes {
		if o.Result.Status == domain.StepWouldApply {
			lines = append(lines, o.Result.Detail)
		}
	}
	for _, it := range res.Items {
		lines = append(lines, fmt.Sprintf("%s: %s:%s", it.Result, it.Key.Root, it.Key.Path))
	}
	return lines
}

func (s *Sync) muted(text string) string { return s.colored(s.palette.TextMuted, text) }
func (s *Sync) warn(text string) string  { return s.colored(s.palette.Warning, text) }

func (s *Sync) colored(c color.Color, text string) string {
	return lipgloss.NewStyle().Foreground(c).Render(text)
}

func directionLabel(d domain.Direction) string {
	switch d {
	case domain.Pull:
		return "Pull"
	case domain.Restore:
		return "Restore"
	}
	return "Push"
}

func changeLabel(t domain.ChangeType) string {
	switch t {
	case domain.Create:
		return "create"
	case domain.Delete:
		return "delete"
	}
	return "update"
}

func badges(it app.PlanItem) []string {
	var out []string
	switch it.Change.Warning {
	case domain.WarnAbsoluteLink:
		out = append(out, "absolute link")
	case domain.WarnLeavesManagedSet:
		out = append(out, "leaves managed set")
	}
	if it.Blocked {
		out = append(out, "blocked")
	}
	return out
}

func timeText(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(timeLayout)
}
