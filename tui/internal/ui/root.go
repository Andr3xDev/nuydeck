package ui

import (
	"context"
	"fmt"
	"slices"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const dryRunBadge = "DRY-RUN"

const (
	sidebarMax     = 30 // sidebar block width cap, border included
	minWidth       = 40
	minHeight      = 8
	noticeMaxLines = 2
)

// wideRowWidth is the panel width below which list rows drop their secondary columns.
const wideRowWidth = 60

// screenBox draws a screen's box; lines must fit contentWidth(width).
func screenBox(th theme, title string, lines []string, width, height int) string {
	return renderBox(th, title, lines, width, height, true)
}

// innerHeight is the number of lines a screen can fill inside its box.
func innerHeight(height int) int { return max(height-borderCells, 1) }

// clip cuts already-styled lines to width cells.
func clip(lines []string, width int) []string {
	style := lipgloss.NewStyle().MaxWidth(max(width, 1))
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = style.Render(l)
	}
	return out
}

// styled applies a style to every line of text.
func styled(style lipgloss.Style, lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = style.Render(l)
	}
	return out
}

type section struct {
	title  string
	screen screen
}

// focus is the one pane that receives keys.
type focus int

const (
	focusSidebar focus = iota
	focusPanel
)

// Root is the top-level Bubble Tea model: sidebar, active panel and keybar.
type Root struct {
	width, height int
	sections      []section
	selected      int
	focus         focus
	showHelp      bool
	dryRun        bool
	svc           *services
	help          help.Model
	cancel        context.CancelFunc
	run           *run
}

// NewRoot builds the shell. variant is the raw theme variant string read by
// the caller; unknown values fall back to Nadir. A manifest error leaves only
// the error screen, so no sync, backup or restore action exists.
func NewRoot(variant string, dryRun bool, deps Deps) *Root {
	th := ForVariant(variant)
	h := help.New()
	h.Styles = help.DefaultStyles(th.Dark)
	ctx, cancel := context.WithCancel(context.Background())
	svc := &services{Deps: deps, ctx: ctx, dryRun: dryRun, th: th, keys: newKeyMap()}
	r := &Root{dryRun: dryRun, svc: svc, help: h, cancel: cancel}
	svc.running = func() bool { return r.run != nil }
	for _, spec := range sectionSpecs {
		if spec.enabled(deps) {
			r.sections = append(r.sections, section{title: spec.title, screen: spec.build(svc)})
		}
	}
	return r
}

// sectionOf finds the section holding a screen of type T.
func sectionOf[T screen](r *Root) (found T, index int, ok bool) {
	for i, s := range r.sections {
		if t, is := s.screen.(T); is {
			return t, i, true
		}
	}
	return found, -1, false
}

// WaitInstall blocks after the program exits until a started run has finished its bookkeeping, up to timeout.
func (r *Root) WaitInstall(timeout time.Duration) {
	if r.run != nil && r.run.started.Load() {
		select {
		case <-r.run.finished:
		case <-time.After(timeout):
		}
	}
}

func (r *Root) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, s := range r.sections {
		if i, ok := s.screen.(initer); ok {
			cmds = append(cmds, i.Init())
		}
	}
	return tea.Batch(cmds...)
}

func (r *Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = msg.Width, msg.Height
		r.help.SetWidth(msg.Width)
		return r, nil
	case tea.KeyPressMsg:
		return r.route(msg)
	case startSyncMsg:
		if s, i, ok := sectionOf[*Sync](r); ok && r.run == nil {
			r.selectSection(i)
			r.focus = focusPanel
			return r, s.startPlan(msg.direction)
		}
		return r, nil
	case execRequestMsg:
		return r, tea.ExecProcess(msg.cmd, msg.finish)
	case runRequest:
		if r.run != nil {
			return r, nil
		}
		workCtx, cancel := context.WithCancel(r.svc.ctx)
		r.run = newRun(msg, cancel)
		return r, r.run.start(r.svc.ctx, workCtx, msg.work)
	case runEventMsg:
		if r.run != nil {
			r.run.add(msg.event)
			return r, r.run.next()
		}
		return r, nil
	case runDoneMsg:
		if r.run == nil {
			return r, nil
		}
		r.run.finish(msg.end)
		if msg.end.msg != nil {
			return r.broadcast(msg.end.msg)
		}
		return r, nil
	}
	return r.broadcast(msg)
}

// broadcast delivers background messages to every screen, since results and
// ticks must land even while another section is selected.
func (r *Root) broadcast(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(r.sections))
	for i := range r.sections {
		r.sections[i].screen, cmds[i] = r.sections[i].screen.Update(msg)
	}
	return r, tea.Batch(cmds...)
}

// route sends a key to one place, in this order: help, run, text capture, global keys, focus.
func (r *Root) route(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keys := r.svc.keys
	switch {
	case r.showHelp && !key.Matches(msg, keys.Quit):
		if key.Matches(msg, keys.Help, keys.Back) {
			r.showHelp = false
		}
		return r, nil
	case r.run != nil:
		return r.routeRun(msg)
	case r.focus == focusPanel && r.panelCapturesText() && msg.String() != "ctrl+c":
		return r.forward(msg)
	case key.Matches(msg, keys.Quit):
		return r.quit()
	case key.Matches(msg, keys.Help):
		r.showHelp = true
		return r, nil
	case key.Matches(msg, keys.Focus):
		r.toggleFocus()
		return r, nil
	case r.focus == focusSidebar:
		return r.routeSidebar(msg)
	}
	return r.routePanel(msg)
}

func (r *Root) quit() (tea.Model, tea.Cmd) {
	for _, s := range r.sections {
		if c, ok := s.screen.(canceller); ok {
			c.Cancel()
		}
		if l, ok := s.screen.(leaver); ok {
			l.Leave()
		}
	}
	r.cancel()
	return r, tea.Quit
}

// routeRun keeps the output pane modal: a live run takes only scrolling, ctrl+c and the reboot enter; a finished one also esc and q.
func (r *Root) routeRun(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keys, run := r.svc.keys, r.run
	ctrlC := msg.String() == "ctrl+c"
	switch {
	case keys.step(msg) != 0:
		run.scrollBy(keys.step(msg))
	case run.done && key.Matches(msg, keys.Quit), ctrlC && run.awaitAck:
		return r.quit()
	case run.done && key.Matches(msg, keys.Back):
		r.run = nil
		return r.broadcast(runClosed{})
	case run.done:
	case ctrlC:
		run.cancel()
	case run.awaitAck && key.Matches(msg, keys.Confirm):
		run.awaitAck = false
		select {
		case run.ack <- struct{}{}:
		default:
		}
	default:
		run.notice = true
	}
	return r, nil
}

func (r *Root) toggleFocus() {
	if r.focus == focusSidebar {
		r.focus = focusPanel
	} else {
		r.focus = focusSidebar
	}
}

func (r *Root) routeSidebar(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keys := r.svc.keys
	if key.Matches(msg, keys.Open) {
		r.focus = focusPanel
	} else if delta := keys.step(msg); delta != 0 {
		r.selectSection(max(min(r.selected+delta, len(r.sections)-1), 0))
		return r, r.active().Enter()
	}
	return r, nil
}

// routePanel delivers only the keys the screen's current stage lists; esc goes back to the sidebar unless the stage claims it.
func (r *Root) routePanel(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keys := r.svc.keys
	switch {
	case key.Matches(msg, r.active().Keys()...):
		return r.forward(msg)
	case key.Matches(msg, keys.Refresh):
		return r, r.active().Enter()
	case key.Matches(msg, keys.Back):
		r.focus = focusSidebar
	}
	return r, nil
}

func (r *Root) active() screen { return r.sections[r.selected].screen }

// selectSection moves the selection and lets the section being left clean up.
func (r *Root) selectSection(i int) {
	if i != r.selected {
		if l, ok := r.active().(leaver); ok {
			l.Leave()
		}
	}
	r.selected = i
}

// panelCapturesText reports whether the active screen is reading typed text, so shell keys such as ? and q must reach it.
func (r *Root) panelCapturesText() bool {
	c, ok := r.active().(textCapturer)
	return ok && c.CapturesText()
}

func (r *Root) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := r.active().Update(msg)
	r.sections[r.selected].screen = next
	return r, cmd
}

func (r *Root) View() tea.View {
	v := tea.NewView(r.render())
	v.AltScreen = true
	return v
}

func (r *Root) render() string {
	if r.width <= 0 || r.height <= 0 {
		return ""
	}
	if r.width < minWidth || r.height < minHeight {
		return lipgloss.NewStyle().MaxWidth(r.width).Render("Terminal too small")
	}
	band := r.renderNoticeBand()
	bodyHeight := r.height - 1 - len(band)
	sidebarCells := min(sidebarMax, r.width*2/5)
	panelWidth := r.width - sidebarCells

	var panel string
	if r.run != nil {
		panel = r.run.view(r.svc.th, panelWidth, bodyHeight)
	} else {
		panel = r.active().View(panelWidth, bodyHeight)
	}
	panel = lipgloss.NewStyle().MaxWidth(panelWidth).MaxHeight(bodyHeight).Render(panel)
	titles := make([]string, len(r.sections))
	for i, s := range r.sections {
		titles[i] = s.title
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, renderSidebar(r.svc.th, titles, r.selected, r.focus == focusPanel, sidebarCells, bodyHeight), panel)
	if r.showHelp {
		body = overlayCentered(body, helpPopup(r.svc.th, r.help, r.helpGroups(), r.width, bodyHeight), r.width, bodyHeight)
	}
	return lipgloss.JoinVertical(lipgloss.Left, slices.Concat([]string{body}, band, []string{r.renderKeybar()})...)
}

// renderNoticeBand wraps the state warning and the active screen's notices under the panel, cut to noticeMaxLines so the panel keeps its rows.
func (r *Root) renderNoticeBand() []string {
	texts := []string{r.svc.StateWarning}
	if n, ok := r.active().(noticer); ok {
		texts = append(texts, n.Notices()...)
	}
	var lines []string
	for _, text := range texts {
		if text != "" {
			lines = append(lines, wrap(text, r.width)...)
		}
	}
	if len(lines) > noticeMaxLines {
		lines = append(lines[:noticeMaxLines-1], fmt.Sprintf("+%d more", len(lines)-noticeMaxLines+1))
	}
	return styled(r.svc.th.warning, lines)
}

// focusedKeys are the bindings valid in the current focus; the keybar and help derive from them.
func (r *Root) focusedKeys() []key.Binding {
	if r.run != nil {
		return r.run.keys(r.svc.keys)
	}
	if r.focus == focusSidebar {
		return r.svc.keys.sidebarKeys()
	}
	return r.svc.keys.panelKeys(r.active().Keys())
}

// helpGroups lists the focused keys, then the shell keys valid everywhere.
func (r *Root) helpGroups() [][]key.Binding {
	return [][]key.Binding{r.focusedKeys(), slices.Concat([]key.Binding{r.svc.keys.Focus}, r.svc.keys.pinnedKeys())}
}

// pinnedKeys are the right-hand keys: a live run answers neither ? nor q, a finished one only q.
func (r *Root) pinnedKeys() []key.Binding {
	switch {
	case r.run == nil:
		return r.svc.keys.pinnedKeys()
	case r.run.done:
		return []key.Binding{r.svc.keys.Quit}
	}
	return nil
}

func (r *Root) renderKeybar() string {
	return keybar(r.svc.th, r.help, r.focusedKeys(), r.pinnedKeys(), r.dryRun, r.width)
}
