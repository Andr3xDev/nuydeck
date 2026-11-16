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

// Drifter streams per-entry drift results; *drift.Service implements it.
type Drifter interface {
	Drift(ctx context.Context, entries []manifest.ResolvedEntry) <-chan drift.DriftResult
}

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
	drifter    Drifter
	entries    []manifest.ResolvedEntry
	state      dashboardInfo
	keys       KeyMap
	palette    Palette
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
	warning  string
	lastSync *state.LastSync
}

func newDashboard(deps Deps, keys KeyMap, palette Palette) *Dashboard {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(lipgloss.NewStyle().Foreground(palette.Accent)))
	return &Dashboard{
		drifter: deps.Drifter,
		entries: deps.Entries,
		state:   dashboardInfo{warning: deps.StateWarning, lastSync: deps.LastSync},
		keys:    keys,
		palette: palette,
		spinner: sp,
		results: map[manifest.Entry]drift.DriftResult{},
	}
}

// start cancels any prior run, bumps the generation and begins a new drift stream.
func (d *Dashboard) start() tea.Cmd {
	if d.cancel != nil {
		d.cancel()
	}
	d.gen++
	d.results = map[manifest.Entry]drift.DriftResult{}
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

func (d *Dashboard) cancelWork() {
	if d.cancel != nil {
		d.cancel()
	}
}

func requestSync(dir plan.Direction) tea.Cmd {
	return func() tea.Msg { return startSyncMsg{direction: dir} }
}

func (d *Dashboard) View(width, height int) string {
	lines := append(strings.Split(d.statusLines(), "\n"), "")
	if len(d.entries) == 0 {
		lines = append(lines, d.muted("No managed entries."))
	}
	detail := d.errorDetail()
	reserved := 0
	if detail != "" {
		reserved = 1
	}
	visible := max(height-2*panelPadY-len(lines)-reserved, 1)
	if detail != "" {
		lines = append(lines, detail)
	}
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

// errorDetail is the cursor row's error flattened to one line, or "" when the row has none.
func (d *Dashboard) errorDetail() string {
	if d.cursor >= len(d.entries) {
		return ""
	}
	r, ok := d.results[d.entries[d.cursor].Entry]
	if !ok || r.State != drift.DriftError || r.Err == nil {
		return ""
	}
	return lipgloss.NewStyle().Foreground(d.palette.Error).Render(strings.Join(strings.Fields(r.Err.Error()), " "))
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
		return lipgloss.NewStyle().Foreground(d.palette.Error).Render("error")
	case drift.DriftDrifted:
		switch r.Missing {
		case drift.MissingInRepo:
			return lipgloss.NewStyle().Foreground(d.palette.Warning).Render("only in live")
		case drift.MissingOnPC:
			return lipgloss.NewStyle().Foreground(d.palette.Warning).Render("not on PC")
		}
		return lipgloss.NewStyle().Foreground(d.palette.Warning).Render(fmt.Sprintf("drifted (%d changes)", r.Changes))
	}
	return lipgloss.NewStyle().Foreground(d.palette.Success).Render("in sync")
}

func (d *Dashboard) muted(s string) string {
	return lipgloss.NewStyle().Foreground(d.palette.TextMuted).Render(s)
}

// ManifestError replaces the sections that depend on an invalid manifest: no operation is offered.
type ManifestError struct {
	err     error
	palette Palette
	// fix names the file to repair; empty means manifest/paths.toml.
	fix string
}

func (m ManifestError) Update(tea.Msg) (Screen, tea.Cmd) { return m, nil }

func (m ManifestError) View(width, height int) string {
	fix := m.fix
	if fix == "" {
		fix = "manifest/paths.toml"
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(m.palette.Error).Render("Invalid manifest")
	return renderPanel(width, height, title+"\n\n"+wrapToPanel(width, m.err.Error())+"\n\nFix "+fix+" and restart.")
}
