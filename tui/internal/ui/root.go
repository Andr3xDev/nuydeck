package ui

import (
	"context"
	"slices"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const dryRunBadge = "DRY-RUN"

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

// WaitInstall blocks after the program exits until a started install has finished its bookkeeping, up to timeout.
func (r *Root) WaitInstall(timeout time.Duration) {
	if in, _, ok := sectionOf[*Install](r); ok {
		in.WaitRun(timeout)
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
		if s, i, ok := sectionOf[*Sync](r); ok {
			r.selectSection(i)
			r.focus = focusPanel
			return r, s.startPlan(msg.direction)
		}
		return r, nil
	case restorePlanMsg:
		if s, i, ok := sectionOf[*Sync](r); ok && msg.err == nil {
			s.showPlan(msg.plan)
			r.selectSection(i)
			r.focus = focusPanel
			return r, nil
		}
	case execRequestMsg:
		return r, tea.ExecProcess(msg.cmd, msg.finish)
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

// route sends a key to one place, in this order: help, text capture, global keys, focus.
// The run modal slots in after help once it exists.
func (r *Root) route(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keys := r.svc.keys
	switch {
	case r.showHelp && !key.Matches(msg, keys.Quit):
		if key.Matches(msg, keys.Help, keys.Back) {
			r.showHelp = false
		}
		return r, nil
	case r.focus == focusPanel && r.panelCapturesText() && msg.String() != "ctrl+c":
		return r.forward(msg)
	case key.Matches(msg, keys.Quit):
		for _, s := range r.sections {
			if c, ok := s.screen.(canceller); ok {
				c.Cancel()
			}
		}
		r.cancel()
		return r, tea.Quit
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
	if r.width <= 0 || r.height <= 1 {
		return ""
	}
	bodyHeight := r.height - 1
	panelWidth := max(r.width-sidebarWidth, 0)

	panel := r.active().View(panelWidth, bodyHeight)
	panel = lipgloss.NewStyle().MaxWidth(panelWidth).MaxHeight(bodyHeight).Render(panel)
	titles := make([]string, len(r.sections))
	for i, s := range r.sections {
		titles[i] = s.title
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, renderSidebar(r.svc.th, titles, r.selected, r.focus == focusPanel, bodyHeight), panel)
	if r.showHelp {
		body = overlayCentered(body, helpPopup(r.svc.th, r.help, r.helpGroups(), r.width, bodyHeight), r.width, bodyHeight)
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, r.renderKeybar())
}

// focusedKeys are the bindings valid in the current focus; the keybar and help derive from them.
func (r *Root) focusedKeys() []key.Binding {
	if r.focus == focusSidebar {
		return r.svc.keys.sidebarKeys()
	}
	return r.svc.keys.panelKeys(r.active().Keys())
}

// helpGroups lists the focused keys, then the shell keys valid everywhere.
func (r *Root) helpGroups() [][]key.Binding {
	return [][]key.Binding{r.focusedKeys(), slices.Concat([]key.Binding{r.svc.keys.Focus}, r.svc.keys.pinnedKeys())}
}

func (r *Root) renderKeybar() string {
	return keybar(r.svc.th, r.help, r.focusedKeys(), r.svc.keys.pinnedKeys(), r.dryRun, r.width)
}
