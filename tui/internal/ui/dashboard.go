package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/theme"
)

const timeLayout = "2006-01-02 15:04"

// Drifter streams per-entry drift results; *app.Service implements it.
type Drifter interface {
	Drift(ctx context.Context, entries []domain.ResolvedEntry) <-chan app.DriftResult
}

// driftEntryMsg carries one entry result; gen drops results of a superseded run.
type driftEntryMsg struct {
	gen    int
	result app.DriftResult
}

type driftDoneMsg struct{ gen int }

// startSyncMsg asks the root to open the Sync section for a direction.
type startSyncMsg struct{ direction domain.Direction }

// Dashboard lists drift per managed entry, computed in the background.
type Dashboard struct {
	drifter    Drifter
	entries    []domain.ResolvedEntry
	state      dashboardInfo
	keys       KeyMap
	palette    theme.Palette
	spinner    spinner.Model
	results    map[domain.Entry]app.DriftResult
	lastBackup *domain.Backup
	cursor     int
	gen        int
	running    bool
	cancel     context.CancelFunc
	stream     <-chan app.DriftResult
}

type dashboardInfo struct {
	warning  string
	lastSync *domain.LastSync
}

func newDashboard(deps Deps, keys KeyMap, palette theme.Palette) *Dashboard {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(lipgloss.NewStyle().Foreground(palette.Accent)))
	return &Dashboard{
		drifter: deps.Drifter,
		entries: deps.Entries,
		state:   dashboardInfo{warning: deps.StateWarning, lastSync: deps.LastSync},
		keys:    keys,
		palette: palette,
		spinner: sp,
		results: map[domain.Entry]app.DriftResult{},
	}
}

// start cancels any prior run, bumps the generation and begins a new drift stream.
func (d *Dashboard) start() tea.Cmd {
	if d.cancel != nil {
		d.cancel()
	}
	d.gen++
	d.results = map[domain.Entry]app.DriftResult{}
	d.lastBackup = nil
	if d.drifter == nil || len(d.entries) == 0 {
		d.running, d.cancel = false, nil
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.running = true
	d.stream = d.drifter.Drift(ctx, d.entries)
	return tea.Batch(waitDrift(d.gen, d.stream), d.spinner.Tick)
}

// waitDrift reads one result and is re-issued by Update until the stream closes (D10).
func waitDrift(gen int, ch <-chan app.DriftResult) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return driftDoneMsg{gen: gen}
		}
		return driftEntryMsg{gen: gen, result: r}
	}
}

func (d *Dashboard) Init() tea.Cmd { return d.start() }

func (d *Dashboard) Update(msg tea.Msg) (Screen, tea.Cmd) {
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
			return d, requestSync(domain.Pull)
		case key.Matches(msg, d.keys.Push):
			return d, requestSync(domain.Push)
		}
	}
	return d, nil
}

// recordSync adopts the last-sync stamp of a really applied sync; a dry-run
// only reports StepWouldApply, so it never changes the shown stamp.
func (d *Dashboard) recordSync(res app.ApplyResult) {
	for _, o := range res.Outcomes {
		if st, ok := o.Step.(domain.SaveStateStep); ok && o.Result.Status == domain.StepOK {
			d.state.lastSync = st.State.LastSync
		}
	}
}

func (d *Dashboard) cancelWork() {
	if d.cancel != nil {
		d.cancel()
	}
}

func requestSync(dir domain.Direction) tea.Cmd {
	return func() tea.Msg { return startSyncMsg{direction: dir} }
}

func (d *Dashboard) View(width, height int) string {
	lines := append(strings.Split(d.statusLines(), "\n"), "")
	if len(d.entries) == 0 {
		lines = append(lines, d.muted("No managed entries."))
	}
	visible := max(height-2*panelPadY-len(lines), 1)
	start := max(min(d.cursor-visible/2, len(d.entries)-visible), 0)
	for i := start; i < min(start+visible, len(d.entries)); i++ {
		e := d.entries[i]
		cursor := "  "
		if i == d.cursor {
			cursor = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%-40s %s", cursor, fmt.Sprintf("%s:%s", e.Root, e.Path), d.entryStatus(e.Entry)))
	}
	return renderPanel(width, height, strings.Join(lines, "\n"))
}

func (d *Dashboard) statusLines() string {
	lines := []string{d.muted("Last sync:   ") + d.lastSyncText(), d.muted("Last backup: ") + d.lastBackupText()}
	if d.state.warning != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(d.palette.Warning).Render("State: "+d.state.warning))
	}
	return strings.Join(lines, "\n")
}

func (d *Dashboard) lastSyncText() string {
	if d.state.lastSync == nil {
		return "never"
	}
	return fmt.Sprintf("%s (%s)", d.state.lastSync.Time.Format(timeLayout), d.state.lastSync.Direction)
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

func (d *Dashboard) entryStatus(e domain.Entry) string {
	r, ok := d.results[e]
	if !ok {
		if d.running {
			return d.spinner.View()
		}
		return d.muted("-")
	}
	switch r.State {
	case app.DriftError:
		return lipgloss.NewStyle().Foreground(d.palette.Error).Render("error: " + r.Err.Error())
	case app.DriftDrifted:
		return lipgloss.NewStyle().Foreground(d.palette.Warning).Render(fmt.Sprintf("drifted (%d changes)", r.Changes))
	}
	return lipgloss.NewStyle().Foreground(d.palette.Success).Render("in sync")
}

func (d *Dashboard) muted(s string) string {
	return lipgloss.NewStyle().Foreground(d.palette.TextMuted).Render(s)
}

// ManifestError replaces every section when the manifest is invalid: no operation is offered.
type ManifestError struct {
	err     error
	palette theme.Palette
}

func (m ManifestError) Update(tea.Msg) (Screen, tea.Cmd) { return m, nil }

func (m ManifestError) View(width, height int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(m.palette.Error).Render("Invalid manifest")
	return renderPanel(width, height, title+"\n\n"+wrapToPanel(width, m.err.Error())+"\n\nFix manifest/paths.toml and restart.")
}
