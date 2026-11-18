package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/references"
)

type linkOpenedMsg struct{ err error }

// References lists the external links with whether each is installed and opens the selected one.
type References struct {
	svc      *shell
	statuses []references.Status
	cursor   int
	err      error
}

func newReferences(svc *shell) *References {
	return &References{svc: svc, statuses: make([]references.Status, len(svc.References.Catalog))}
}

func (r *References) Title() string { return "References" }

func (r *References) Enter() tea.Cmd {
	for i, ref := range r.svc.References.Catalog {
		r.statuses[i] = r.svc.References.Status(ref)
	}
	return nil
}

func (r *References) Keys() []key.Binding {
	k := r.svc.keys
	return []key.Binding{k.Up, k.Down, k.Link}
}

func (r *References) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case linkOpenedMsg:
		r.err = msg.err
	case tea.KeyPressMsg:
		return r, r.handleKey(msg)
	}
	return r, nil
}

func (r *References) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := r.svc.keys
	catalog := r.svc.References.Catalog
	switch {
	case key.Matches(msg, k.Up):
		r.cursor = max(r.cursor-1, 0)
	case key.Matches(msg, k.Down):
		r.cursor = max(min(r.cursor+1, len(catalog)-1), 0)
	case key.Matches(msg, k.Link) && r.cursor < len(catalog):
		open, ctx, url := r.svc.References.Open, r.svc.ctx, catalog[r.cursor].URL
		r.err = nil
		return func() tea.Msg { return linkOpenedMsg{err: open(ctx, url)} }
	}
	return nil
}

func statusText(s references.Status) string {
	switch s {
	case references.StatusPresent:
		return "present"
	case references.StatusAbsent:
		return "absent"
	}
	return "unknown"
}

func (r *References) View(width, height int) string {
	th, cw := r.svc.th, contentWidth(width)
	catalog := r.svc.References.Catalog
	var lines []string
	if r.err != nil {
		lines = styled(th.err, wrap(r.err.Error(), cw))
	}
	if len(catalog) == 0 {
		return screenBox(th, r.Title(), append(lines, th.muted.Render("No references.")), width, height)
	}
	rows := make([]listRow, len(catalog))
	for i, ref := range catalog {
		rows[i] = listRow{text: fmt.Sprintf("%-7s %s  %s", statusText(r.statuses[i]), ref.Name, ref.Description)}
	}
	list, at := listLines(th, rows, r.cursor, cw)
	lines = append(lines, windowAround(list, at, max(innerHeight(height)-len(lines), 1))...)
	return screenBox(th, r.Title(), lines, width, height)
}
