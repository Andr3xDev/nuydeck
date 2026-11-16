package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

const (
	manifestFile = "manifest/paths.toml"
	catalogFiles = "manifest/packages.toml, hardware.toml or secrets.toml"
)

// errorScreen replaces the sections that depend on an invalid manifest: no operation is offered.
type errorScreen struct {
	err error
	th  theme
	// fix names the file to repair; empty means manifest/paths.toml.
	fix string
}

func newErrorScreen(svc *services, err error, fix string) errorScreen {
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
	lines := styled(m.th.err, wrap(m.err.Error(), contentWidth(width)))
	lines = append(lines, "")
	lines = append(lines, wrap("Fix "+fix+" and restart.", contentWidth(width))...)
	return screenBox(m.th, m.Title(), lines, width, height)
}
