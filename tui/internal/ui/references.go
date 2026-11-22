package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/references"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
)

type linkOpenedMsg struct{ err error }

type referenceStatusesMsg []references.Status

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
	catalog, status := r.svc.References.Catalog, r.svc.References.Status
	return func() tea.Msg {
		statuses := make([]references.Status, len(catalog))
		for i, ref := range catalog {
			statuses[i] = status(ref)
		}
		return referenceStatusesMsg(statuses)
	}
}

func (r *References) Keys() []key.Binding {
	k := r.svc.keys
	return append(k.Move(), k.Link)
}

func (r *References) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case referenceStatusesMsg:
		r.statuses = msg
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
	case k.Step(msg) != 0:
		r.cursor = kit.MoveCursor(r.cursor, k.Step(msg), len(catalog))
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
	th, cw := r.svc.th, kit.ContentWidth(width)
	catalog := r.svc.References.Catalog
	var lines []string
	if r.err != nil {
		lines = styled(th.Err, kit.Wrap(r.err.Error(), cw))
	}
	if len(catalog) == 0 {
		return screenBox(th, r.Title(), append(lines, th.Muted.Render("No references.")), width, height)
	}
	rows := make([]kit.ListRow, len(catalog))
	for i, ref := range catalog {
		rows[i] = kit.ListRow{Text: fmt.Sprintf("%-7s %s  %s", statusText(r.statuses[i]), ref.Name, ref.Description)}
	}
	list, at := kit.ListLines(th, rows, r.cursor, cw)
	lines = append(lines, kit.WindowAround(list, at, max(innerHeight(height)-len(lines), 1))...)
	return screenBox(th, r.Title(), lines, width, height)
}
