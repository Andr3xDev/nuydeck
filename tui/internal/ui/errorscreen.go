package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
)

const manifestFile = "manifest/paths.toml"

// errorScreen replaces the sections that depend on an invalid manifest: no operation is offered.
type errorScreen struct {
	err error
	th  kit.Theme
	// fix names the file to repair; empty means manifest/paths.toml.
	fix string
}

func newErrorScreen(svc *shell, err error, fix string) errorScreen {
	return errorScreen{err: err, th: svc.th, fix: fix}
}

func (m errorScreen) Title() string                    { return "Invalid manifest" }
func (m errorScreen) Enter() tea.Cmd                   { return nil }
func (m errorScreen) Update(tea.Msg) (screen, tea.Cmd) { return m, nil }
func (m errorScreen) Keys() []key.Binding              { return nil }

func (m errorScreen) View(width, height int) string {
	fix := m.fix
	if fix == "" {
		fix = manifestFile
	}
	lines := styled(m.th.Err, kit.Wrap(m.err.Error(), kit.ContentWidth(width)))
	lines = append(lines, "")
	lines = append(lines, kit.Wrap("Fix "+fix+" and restart.", kit.ContentWidth(width))...)
	return screenBox(m.th, m.Title(), lines, width, height)
}
