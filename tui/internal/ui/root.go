package ui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/theme"
)

const (
	sidebarWidth = 20
	dryRunBadge  = "DRY-RUN"
)

const (
	panelPadX = 2
	panelPadY = 1
)

// renderPanel pads content into a width x height box. Lines are clipped, never
// wrapped, so long paths cannot push the box past its height.
func renderPanel(width, height int, content string) string {
	return lipgloss.NewStyle().Width(width).Height(height).Padding(panelPadY, panelPadX).
		Render(lipgloss.NewStyle().MaxWidth(panelInnerWidth(width)).Render(content))
}

func panelInnerWidth(width int) int { return max(width-2*panelPadX, 1) }

// wrapToPanel wraps text to the panel's inner width so a long message stays
// readable instead of being clipped by renderPanel.
func wrapToPanel(width int, text string) string {
	return lipgloss.NewStyle().Width(panelInnerWidth(width)).Render(text)
}

// canceller is implemented by screens with background work to stop on quit.
type canceller interface{ cancelWork() }

// Screen is one sidebar section's panel. Later tasks replace the placeholders.
type Screen interface {
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View(width, height int) string
}

type section struct {
	title  string
	screen Screen
}

type placeholder struct{ title string }

func (p placeholder) Update(tea.Msg) (Screen, tea.Cmd) { return p, nil }

func (p placeholder) View(width, height int) string {
	return renderPanel(width, height, p.title)
}

const syncSection = 1

// Deps is what the shell needs from the app layer and startup.
type Deps struct {
	Drifter       Drifter
	Syncer        Syncer
	Backups       BackupManager
	Entries       []domain.ResolvedEntry
	StateWarning  string
	LastSync      *domain.LastSync
	ManifestError error
}

// initer is implemented by screens that start work when the program starts.
type initer interface{ Init() tea.Cmd }

// Root is the top-level Bubble Tea model: sidebar, active panel and keybar.
type Root struct {
	width, height int
	sections      []section
	selected      int
	panelFocus    bool
	showHelp      bool
	dryRun        bool
	keys          KeyMap
	help          help.Model
	palette       theme.Palette
}

// NewRoot builds the shell. variant is the raw theme variant string read by
// the caller; unknown values fall back to Nadir. A manifest error leaves only
// the error screen, so no sync, backup or restore action exists.
func NewRoot(variant string, dryRun bool, deps Deps) Root {
	palette := theme.ForVariant(variant)
	h := help.New()
	h.Styles = help.DefaultStyles(palette.Dark)
	keys := DefaultKeyMap()
	sections := []section{
		{"Dashboard", newDashboard(deps, keys, palette)},
		{"Sync", newSync(deps, dryRun, keys, palette)},
		{"Backups", newBackups(deps, keys, palette)},
	}
	if deps.ManifestError != nil {
		sections = []section{{"Manifest", ManifestError{err: deps.ManifestError, palette: palette}}}
	}
	return Root{
		sections: sections,
		dryRun:   dryRun,
		keys:     keys,
		help:     h,
		palette:  palette,
	}
}

func (r Root) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, s := range r.sections {
		if i, ok := s.screen.(initer); ok {
			cmds = append(cmds, i.Init())
		}
	}
	return tea.Batch(cmds...)
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = msg.Width, msg.Height
		r.help.SetWidth(msg.Width)
		return r, nil
	case tea.KeyPressMsg:
		return r.handleKey(msg)
	case startSyncMsg:
		r.selected, r.panelFocus = syncSection, true
		if s, ok := r.sections[syncSection].screen.(*Sync); ok {
			return r, s.startPlan(msg.direction)
		}
		return r, nil
	case restorePlanMsg:
		if s, ok := r.sections[syncSection].screen.(*Sync); ok && msg.err == nil {
			s.showPlan(msg.plan)
			r.selected, r.panelFocus = syncSection, true
			return r, nil
		}
	}
	return r.broadcast(msg)
}

// broadcast delivers background messages to every screen, since results and
// ticks must land even while another section is selected.
func (r Root) broadcast(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(r.sections))
	for i := range r.sections {
		r.sections[i].screen, cmds[i] = r.sections[i].screen.Update(msg)
	}
	return r, tea.Batch(cmds...)
}

func (r Root) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case r.panelModal() && !r.showHelp && (key.Matches(msg, r.keys.Back) || msg.String() == "q"):
		return r.forward(msg)
	case key.Matches(msg, r.keys.Quit):
		for _, s := range r.sections {
			if c, ok := s.screen.(canceller); ok {
				c.cancelWork()
			}
		}
		return r, tea.Quit
	case key.Matches(msg, r.keys.Help):
		r.showHelp = !r.showHelp
		return r, nil
	case r.showHelp:
		if key.Matches(msg, r.keys.Back) {
			r.showHelp = false
		}
		return r, nil
	case key.Matches(msg, r.keys.Focus):
		r.panelFocus = !r.panelFocus
		return r, nil
	case !r.panelFocus && key.Matches(msg, r.keys.Up):
		r.selected = max(r.selected-1, 0)
		return r, r.enterSelected()
	case !r.panelFocus && key.Matches(msg, r.keys.Down):
		r.selected = min(r.selected+1, len(r.sections)-1)
		return r, r.enterSelected()
	}
	return r.forward(msg)
}

// enterSelected lets the newly selected screen refresh itself.
func (r Root) enterSelected() tea.Cmd {
	if e, ok := r.sections[r.selected].screen.(interface{ enter() tea.Cmd }); ok {
		return e.enter()
	}
	return nil
}

// panelModal reports whether the active screen has a dialog open, so q and esc close it first.
func (r Root) panelModal() bool {
	m, ok := r.sections[r.selected].screen.(interface{ Modal() bool })
	return ok && m.Modal()
}

func (r Root) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	screen, cmd := r.sections[r.selected].screen.Update(msg)
	r.sections[r.selected].screen = screen
	return r, cmd
}

func (r Root) View() tea.View {
	v := tea.NewView(r.render())
	v.AltScreen = true
	return v
}

func (r Root) render() string {
	if r.width <= 0 || r.height <= 1 {
		return ""
	}
	bodyHeight := r.height - 1
	panelWidth := max(r.width-sidebarWidth, 0)

	panel := r.sections[r.selected].screen.View(panelWidth, bodyHeight)
	if r.showHelp {
		panel = renderPanel(panelWidth, bodyHeight, r.help.FullHelpView(r.keys.FullHelp()))
	}
	panel = lipgloss.NewStyle().MaxWidth(panelWidth).MaxHeight(bodyHeight).Render(panel)
	body := lipgloss.JoinHorizontal(lipgloss.Top, r.renderSidebar(bodyHeight), panel)
	return lipgloss.JoinVertical(lipgloss.Left, body, r.renderKeybar())
}

func (r Root) renderSidebar(height int) string {
	normal := lipgloss.NewStyle().Foreground(r.palette.TextSecondary).Padding(0, 1)
	active := lipgloss.NewStyle().Foreground(r.palette.OnAccent).Background(r.palette.Accent).Padding(0, 1)
	if !r.panelFocus {
		active = active.Bold(true)
	}

	lines := make([]string, len(r.sections))
	for i, s := range r.sections {
		style := normal
		if i == r.selected {
			style = active
		}
		lines[i] = style.Width(sidebarWidth).Render(s.title)
	}
	return lipgloss.NewStyle().Width(sidebarWidth).Height(height).
		BorderStyle(lipgloss.NormalBorder()).BorderRight(true).BorderForeground(r.palette.Border).
		Render(strings.Join(lines, "\n"))
}

func (r Root) renderKeybar() string {
	badge := ""
	if r.dryRun {
		badge = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Foreground(r.palette.OnAccent).Background(r.palette.Warning).Render(dryRunBadge)
	}
	hint := r.help.ShortHelpView(r.keys.ShortHelp())
	gap := max(r.width-lipgloss.Width(hint)-lipgloss.Width(badge), 0)
	return hint + strings.Repeat(" ", gap) + badge
}
