package ui

import (
	"context"
	"strings"
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

// Root is the top-level Bubble Tea model: sidebar, active panel and keybar.
type Root struct {
	width, height int
	sections      []section
	selected      int
	panelFocus    bool
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
	svc := &services{Deps: deps, ctx: ctx, dryRun: dryRun, th: th, keys: DefaultKeyMap()}
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
		return r.handleKey(msg)
	case startSyncMsg:
		if s, i, ok := sectionOf[*Sync](r); ok {
			r.selectSection(i)
			r.panelFocus = true
			return r, s.startPlan(msg.direction)
		}
		return r, nil
	case restorePlanMsg:
		if s, i, ok := sectionOf[*Sync](r); ok && msg.err == nil {
			s.showPlan(msg.plan)
			r.selectSection(i)
			r.panelFocus = true
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

func (r *Root) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keys := r.svc.keys
	switch {
	case r.panelFocus && r.panelCapturesText() && msg.String() != "ctrl+c":
		return r.forward(msg)
	case r.panelFocus && modal(r.active()) && !r.showHelp && (key.Matches(msg, keys.Back) || msg.String() == "q"):
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
		r.showHelp = !r.showHelp
		return r, nil
	case r.showHelp:
		if key.Matches(msg, keys.Back) {
			r.showHelp = false
		}
		return r, nil
	case key.Matches(msg, keys.Focus):
		r.panelFocus = !r.panelFocus
		return r, nil
	case !r.panelFocus && key.Matches(msg, keys.Up):
		r.selectSection(max(r.selected-1, 0))
		return r, r.active().Enter()
	case !r.panelFocus && key.Matches(msg, keys.Down):
		r.selectSection(min(r.selected+1, len(r.sections)-1))
		return r, r.active().Enter()
	}
	return r.forward(msg)
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
	if r.showHelp {
		helpText := lipgloss.NewStyle().MaxWidth(contentWidth(panelWidth)).Render(r.help.FullHelpView(r.svc.keys.FullHelp()))
		panel = screenBox(r.svc.th, "Help", strings.Split(helpText, "\n"), panelWidth, bodyHeight)
	}
	panel = lipgloss.NewStyle().MaxWidth(panelWidth).MaxHeight(bodyHeight).Render(panel)
	titles := make([]string, len(r.sections))
	for i, s := range r.sections {
		titles[i] = s.title
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, renderSidebar(r.svc.th, titles, r.selected, r.panelFocus, bodyHeight), panel)
	return lipgloss.JoinVertical(lipgloss.Left, body, r.renderKeybar())
}

func (r *Root) renderKeybar() string {
	badge := ""
	if r.dryRun {
		badge = r.svc.th.badge.Render(dryRunBadge)
	}
	hint := r.help.ShortHelpView(r.svc.keys.ShortHelp())
	gap := max(r.width-lipgloss.Width(hint)-lipgloss.Width(badge), 0)
	return hint + strings.Repeat(" ", gap) + badge
}
