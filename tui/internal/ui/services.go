package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

const (
	userScopeLabel   = "--user"
	noTerminalText   = "no terminal available to open the logs"
	notFoundBadge    = "not found"
	unknownBadge     = "unknown"
	maskedBadge      = "masked"
	unavailableState = "-"
)

// scopeRead is the outcome of reading one scope; err leaves its rows unknown.
type scopeRead struct {
	states map[string]services.UnitState
	err    error
}

type servicesReadMsg struct {
	scope plan.Scope
	seq   int // read generation; a reply older than the latest request is dropped
	read  scopeRead
}

type logsClosedMsg struct{ err error }

// serviceAction binds a key to the systemctl action it plans.
type serviceAction struct {
	binding key.Binding
	action  plan.Action
}

// Services lists the catalog units with their live state and plans actions on them through the flow.
type Services struct {
	svc    *shell
	flow   *flow
	rows   []services.Entry // system units first, then user units, each in catalog order
	reads  map[plan.Scope]scopeRead
	cursor int
	notice string
	err    error

	readSeq       map[plan.Scope]int
	ownRunPending bool // a run started here is open, so its close needs a reload
}

func newServices(svc *shell) *Services {
	rows := slices.Clone(svc.Services.Catalog)
	slices.SortStableFunc(rows, func(a, b services.Entry) int { return scopeRank(a.Scope) - scopeRank(b.Scope) })
	return &Services{svc: svc, rows: rows, reads: map[plan.Scope]scopeRead{}, readSeq: map[plan.Scope]int{}}
}

func scopeRank(scope plan.Scope) int {
	if scope == plan.ScopeUser {
		return 1
	}
	return 0
}

func (s *Services) Title() string  { return "Services" }
func (s *Services) Enter() tea.Cmd { return s.load() }

func (s *Services) wired() bool { return s.svc.Services.Plan != nil && s.svc.RunPlan != nil }

func (s *Services) actions() []serviceAction {
	k := s.svc.keys
	return []serviceAction{
		{described(k.Edit, "enable"), plan.ActionEnable},
		{described(k.Delete, "disable"), plan.ActionDisable},
		{k.Start, plan.ActionStart},
		{k.Stop, plan.ActionStop},
		{k.Restart, plan.ActionRestart},
		{k.Mask, plan.ActionMask},
		{k.Unmask, plan.ActionUnmask},
	}
}

// Keys claims esc while the flow is open, so esc steps back instead of leaving the panel.
func (s *Services) Keys() []key.Binding {
	if s.flow.active() {
		return s.flow.Keys()
	}
	k := s.svc.keys
	keys := append(k.move(), k.Refresh)
	if s.wired() {
		for _, a := range s.actions() {
			keys = append(keys, a.binding)
		}
	}
	return append(keys, k.Logs)
}

// load reads each scope that has units; every read is one systemctl call.
func (s *Services) load() tea.Cmd {
	var cmds []tea.Cmd
	for _, scope := range []plan.Scope{plan.ScopeSystem, plan.ScopeUser} {
		var units []string
		for _, e := range s.rows {
			if e.Scope == scope {
				units = append(units, e.Unit)
			}
		}
		if len(units) == 0 {
			continue
		}
		state, ctx := s.svc.Services.State, s.svc.ctx
		s.readSeq[scope]++
		seq := s.readSeq[scope]
		cmds = append(cmds, func() tea.Msg {
			states, err := state(ctx, scope, units)
			return servicesReadMsg{scope: scope, seq: seq, read: scopeRead{states: states, err: err}}
		})
	}
	return tea.Batch(cmds...)
}

func (s *Services) selected() (services.Entry, bool) {
	if s.cursor >= len(s.rows) {
		return services.Entry{}, false
	}
	return s.rows[s.cursor], true
}

// stateOf is false while the unit's scope has not been read or failed to read.
func (s *Services) stateOf(e services.Entry) (services.UnitState, bool) {
	read, ok := s.reads[e.Scope]
	if !ok || read.err != nil {
		return services.UnitState{}, false
	}
	return read.states[e.Unit], true
}

func (s *Services) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case servicesReadMsg:
		if msg.seq == s.readSeq[msg.scope] {
			s.reads[msg.scope] = msg.read
		}
	case logsClosedMsg:
		s.err = msg.err
	case runClosed:
		if s.ownRunPending {
			s.ownRunPending = false
			return s, s.load()
		}
	case plannedMsg:
		if s.flow.active() {
			return s, s.flow.Update(msg)
		}
	case tea.KeyPressMsg:
		if s.flow.active() {
			return s, s.flow.Update(msg)
		}
		return s, s.listKey(msg)
	}
	return s, nil
}

func (s *Services) listKey(msg tea.KeyPressMsg) tea.Cmd {
	k := s.svc.keys
	if delta := k.step(msg); delta != 0 {
		s.cursor = moveCursor(s.cursor, delta, len(s.rows))
		return nil
	}
	if key.Matches(msg, k.Refresh) {
		return s.load()
	}
	entry, ok := s.selected()
	if !ok {
		return nil
	}
	if key.Matches(msg, k.Logs) {
		return s.logs(entry)
	}
	if !s.wired() {
		return nil
	}
	for _, a := range s.actions() {
		if key.Matches(msg, a.binding) {
			return s.act(entry, a.action)
		}
	}
	return nil
}

// act starts the flow for one action; a unit that is not installed, or whose state is unknown, gets a message instead.
func (s *Services) act(entry services.Entry, action plan.Action) tea.Cmd {
	s.notice, s.err = "", nil
	state, known := s.stateOf(entry)
	switch {
	case !known:
		s.notice = fmt.Sprintf("%s: state is unknown, refresh with r", entry.Unit)
		return nil
	case !state.Found:
		s.notice = fmt.Sprintf("%s: %s, nothing to %s", entry.Unit, notFoundBadge, action)
		return nil
	}
	req := services.Request{Unit: entry.Unit, Scope: entry.Scope, Action: action}
	deps := s.svc.Services
	s.flow = newFlow(s.svc, flowSource{
		Title: fmt.Sprintf("%s %s", action, entry.Unit),
		Plan: func(ctx context.Context, _ []string) (staged, error) {
			p, err := deps.Plan(ctx, req, state)
			if err != nil {
				return nil, err
			}
			return newPlanStaged(p), nil
		},
		Run: func(p plan.Plan) runRequest {
			s.ownRunPending = true
			return planRunRequest(s.svc, p)
		},
	})
	return s.flow.start()
}

// logs hands the terminal to the journal pager; the output is never read, so no journal text reaches the TUI.
func (s *Services) logs(entry services.Entry) tea.Cmd {
	s.notice, s.err = "", nil
	cmd, err := services.LogsCommand(entry)
	if err != nil {
		s.err = err
		return nil
	}
	if s.svc.Terminal == nil {
		s.notice = noTerminalText
		return nil
	}
	terminal, ctx := s.svc.Terminal, s.svc.ctx
	return func() tea.Msg { return logsClosedMsg{err: terminal.Run(ctx, cmd)} }
}

func (s *Services) View(width, height int) string {
	if s.flow.active() {
		return s.flow.box(s.Title(), width, height)
	}
	th, cw := s.svc.th, contentWidth(width)
	var lines []string
	for _, scope := range []plan.Scope{plan.ScopeSystem, plan.ScopeUser} {
		if read := s.reads[scope]; read.err != nil {
			lines = append(lines, styled(th.err, wrap(read.err.Error(), cw))...)
		}
	}
	if s.err != nil {
		lines = append(lines, styled(th.err, wrap(s.err.Error(), cw))...)
	}
	if s.notice != "" {
		lines = append(lines, styled(th.warning, wrap(s.notice, cw))...)
	}
	if len(s.rows) == 0 {
		return screenBox(th, s.Title(), append(lines, th.muted.Render("No managed services.")), width, height)
	}
	lines = append(lines, th.title.Render(fmt.Sprintf("%d managed services", len(s.rows))))
	rows := make([]tableRow, len(s.rows))
	for i, e := range s.rows {
		rows[i] = s.row(e)
	}
	header, list, at := tableLines(th, serviceColumns, rows, s.cursor, cw)
	lines = append(lines, header)
	room := max(innerHeight(height)-len(lines)-2, 1)
	lines = append(lines, windowAround(list, at, room)...)
	if desc := s.rows[s.cursor].Description; desc != "" {
		lines = append(lines, "", th.muted.Render(truncate(sanitizeLine(desc), cw)))
	}
	return screenBox(th, s.Title(), lines, width, height)
}

var serviceColumns = []column{
	{title: "Unit"},
	{title: "Scope", priority: 3},
	{title: "Enabled", priority: 2},
	{title: "Active", priority: 2},
	{title: "Flags", priority: 1},
}

// row shows the unit, its scope, the enabled and active state and the flags that need attention.
func (s *Services) row(e services.Entry) tableRow {
	scope := string(e.Scope)
	if e.Scope == plan.ScopeUser {
		scope = userScopeLabel
	}
	enabled, active := unavailableState, unavailableState
	var flags []string
	if e.Critical {
		flags = append(flags, "critical")
	}
	state, known := s.stateOf(e)
	switch {
	case !known:
		enabled, active = unknownBadge, unknownBadge
	case !state.Found:
		flags = append(flags, notFoundBadge)
	default:
		enabled, active = string(state.Enabled), string(state.Active)
		if state.Masked && enabled != maskedBadge {
			flags = append(flags, maskedBadge)
		}
		if state.Active == services.ActiveFailed {
			flags = append(flags, "failed")
		}
	}
	return tableRow{cells: []string{e.Unit, scope, enabled, active, strings.Join(flags, ", ")}}
}
