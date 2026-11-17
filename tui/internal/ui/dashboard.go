package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/drift"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const timeLayout = "2006-01-02 15:04"

// driftEntryMsg carries one entry result; gen drops results of a superseded run.
type driftEntryMsg struct {
	gen    int
	result drift.DriftResult
}

type driftDoneMsg struct{ gen int }

// startSyncMsg asks the root to open the Sync section for a direction.
type startSyncMsg struct{ direction plan.Direction }

// Dashboard lists drift per managed entry, computed in the background.
type Dashboard struct {
	ctx        context.Context
	drift      func(ctx context.Context, entries []manifest.ResolvedEntry) <-chan drift.DriftResult
	entries    []manifest.ResolvedEntry
	state      dashboardInfo
	keys       keyMap
	th         theme
	spinner    spinner.Model
	results    map[manifest.Entry]drift.DriftResult
	lastBackup *backup.Backup
	cursor     int
	gen        int
	running    bool
	cancel     context.CancelFunc
	stream     <-chan drift.DriftResult
}

type dashboardInfo struct {
	lastSync *state.LastSync
}

func newDashboard(svc *services) *Dashboard {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(svc.th.accent))
	return &Dashboard{
		ctx:     svc.ctx,
		drift:   svc.Drift,
		entries: svc.Entries,
		state:   dashboardInfo{lastSync: svc.LastSync},
		keys:    svc.keys,
		th:      svc.th,
		spinner: sp,
		results: map[manifest.Entry]drift.DriftResult{},
	}
}

func (d *Dashboard) Title() string  { return "Dashboard" }
func (d *Dashboard) Enter() tea.Cmd { return nil }

func (d *Dashboard) Keys() []key.Binding {
	return []key.Binding{d.keys.Up, d.keys.Down, d.keys.Refresh, d.keys.Pull, d.keys.Push}
}

// start cancels any prior run, bumps the generation and begins a new drift stream.
func (d *Dashboard) start() tea.Cmd {
	if d.cancel != nil {
		d.cancel()
	}
	d.gen++
	d.results = map[manifest.Entry]drift.DriftResult{}
	d.lastBackup = nil
	if d.drift == nil || len(d.entries) == 0 {
		d.running, d.cancel = false, nil
		return nil
	}
	ctx, cancel := context.WithCancel(d.ctx)
	d.cancel = cancel
	d.running = true
	d.stream = d.drift(ctx, d.entries)
	return tea.Batch(waitDrift(d.gen, d.stream), d.spinner.Tick)
}

// waitDrift reads one result and is re-issued by Update until the stream closes (D10).
func waitDrift(gen int, ch <-chan drift.DriftResult) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return driftDoneMsg{gen: gen}
		}
		return driftEntryMsg{gen: gen, result: r}
	}
}

func (d *Dashboard) Init() tea.Cmd { return d.start() }

func (d *Dashboard) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case driftEntryMsg:
		if msg.gen != d.gen {
			return d, nil
		}
		d.results[msg.result.Entry.Entry] = msg.result
		if msg.result.LastBackup != nil {
			d.lastBackup = msg.result.LastBackup
		}
		return d, waitDrift(msg.gen, d.stream)
	case driftDoneMsg:
		if msg.gen == d.gen {
			d.running = false
		}
		return d, nil
	case applyDoneMsg:
		d.recordSync(msg.result)
		return d, d.start()
	case spinner.TickMsg:
		if !d.running {
			return d, nil
		}
		var cmd tea.Cmd
		d.spinner, cmd = d.spinner.Update(msg)
		return d, cmd
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keys.Refresh):
			return d, d.start()
		case key.Matches(msg, d.keys.Up):
			d.cursor = max(d.cursor-1, 0)
		case key.Matches(msg, d.keys.Down):
			d.cursor = max(min(d.cursor+1, len(d.entries)-1), 0)
		case key.Matches(msg, d.keys.Pull):
			return d, requestSync(plan.Pull)
		case key.Matches(msg, d.keys.Push):
			return d, requestSync(plan.Push)
		}
	}
	return d, nil
}

// recordSync adopts the last-sync stamp of a really applied sync; a dry-run
// only reports StepWouldApply, so it never changes the shown stamp.
func (d *Dashboard) recordSync(res cfgsync.ApplyResult) {
	for _, o := range res.Outcomes {
		if st, ok := o.Step.(plan.SaveStateStep); ok && o.Result.Status == plan.StepOK {
			d.state.lastSync = st.State.LastSync
		}
	}
}

func (d *Dashboard) Cancel() {
	if d.cancel != nil {
		d.cancel()
	}
}

func requestSync(dir plan.Direction) tea.Cmd {
	return func() tea.Msg { return startSyncMsg{direction: dir} }
}

func (d *Dashboard) View(width, height int) string {
	cw := contentWidth(width)
	lines := append(d.statusLines(), "")
	if len(d.entries) == 0 {
		lines = append(lines, d.muted("No managed entries."))
	}
	detail := d.errorDetail(cw)
	reserved := 0
	if detail != "" {
		reserved = 1
	}
	visible := max(innerHeight(height)-len(lines)-reserved, 1)
	if detail != "" {
		lines = append(lines, detail)
	}
	rows := make([]string, len(d.entries))
	for i, e := range d.entries {
		rows[i] = d.row(i, e, width)
	}
	lines = append(lines, windowAround(rows, d.cursor, visible)...)
	return screenBox(d.th, d.Title(), clip(lines, cw), width, height)
}

// row is the entry path with its status right-aligned; narrow panels drop the status.
func (d *Dashboard) row(i int, e manifest.ResolvedEntry, width int) string {
	cursor := "  "
	if i == d.cursor {
		cursor = "> "
	}
	cw := contentWidth(width)
	path := sanitizeLine(fmt.Sprintf("%s:%s", e.Root, e.Path))
	if width < wideRowWidth {
		return cursor + truncate(path, cw-len(cursor))
	}
	status := d.entryStatus(e.Entry)
	pathWidth := max(cw-len(cursor)-1-lipgloss.Width(status), 1)
	path = truncate(path, pathWidth)
	return cursor + path + strings.Repeat(" ", max(pathWidth-lipgloss.Width(path), 0)) + " " + status
}

// errorDetail is the cursor row's error flattened to one line, or "" when the row has none.
func (d *Dashboard) errorDetail(cw int) string {
	if d.cursor >= len(d.entries) {
		return ""
	}
	r, ok := d.results[d.entries[d.cursor].Entry]
	if !ok || r.State != drift.DriftError || r.Err == nil {
		return ""
	}
	return d.th.err.Render(truncate(sanitizeLine(strings.Join(strings.Fields(r.Err.Error()), " ")), cw))
}

func (d *Dashboard) statusLines() []string {
	return []string{d.muted("Last sync:   ") + d.lastSyncText(), d.muted("Last backup: ") + d.lastBackupText()}
}

func (d *Dashboard) lastSyncText() string {
	if d.state.lastSync == nil {
		return "never"
	}
	return sanitizeLine(fmt.Sprintf("%s (%s)", d.state.lastSync.Time.Format(timeLayout), d.state.lastSync.Direction))
}

func (d *Dashboard) lastBackupText() string {
	switch {
	case d.lastBackup != nil:
		return d.lastBackup.Created.Format(timeLayout)
	case d.running:
		return d.spinner.View()
	}
	return "none"
}

func (d *Dashboard) entryStatus(e manifest.Entry) string {
	r, ok := d.results[e]
	if !ok {
		if d.running {
			return d.spinner.View()
		}
		return d.muted("-")
	}
	switch r.State {
	case drift.DriftError:
		return d.th.err.Render("error")
	case drift.DriftDrifted:
		switch r.Missing {
		case drift.MissingInRepo:
			return d.th.warning.Render("only in live")
		case drift.MissingOnPC:
			return d.th.warning.Render("not on PC")
		}
		return d.th.warning.Render(fmt.Sprintf("drifted (%d changes)", r.Changes))
	}
	return d.th.ok.Render("in sync")
}

func (d *Dashboard) muted(s string) string { return d.th.muted.Render(s) }
