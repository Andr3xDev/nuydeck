package ui

import (
	"context"
	"errors"
	"slices"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// logLineCap is how many output lines the run page keeps.
const logLineCap = 2000

var errInstallUnavailable = errors.New("install is not available")

type installPage int

const (
	pageProfile installPage = iota
	pageHardware
	pageSelection
	pageSecrets
	pageSummary
	pageRun
)

var profileChoices = []string{manifest.ProfileMinimal, manifest.ProfileFull, install.ProfileCustom}

type stepStatus int

const (
	statusPending stepStatus = iota
	statusRunning
	statusDone
	statusSkipped
	statusFailed
)

type gpuDetectedMsg struct {
	ids      []string
	detected bool
}

type installPlanMsg struct {
	gen  int
	plan install.InstallPlan
	err  error
}

type installStream struct {
	events <-chan plan.Event
	result <-chan install.InstallResult
}

type installStartedMsg struct {
	gen    int
	stream installStream
}

type installEventMsg struct {
	gen   int
	event plan.Event
}

type installDoneMsg struct {
	gen    int
	result install.InstallResult
}

type selectionRow struct {
	group string
	name  string // empty for a group header
}

// Install is the guided install wizard: profile, hardware, selection, secrets, summary, run.
type Install struct {
	ctx  context.Context
	deps InstallDeps
	keys keyMap
	th   theme

	page   installPage
	draft  install.Draft
	cursor int
	gen    int
	cancel context.CancelFunc

	runDone chan struct{}

	hwLoading bool
	hwLoaded  bool
	detected  bool

	selected map[string]bool
	seeded   bool
	rows     []selectionRow

	secretIdx    int
	secretInput  textinput.Model
	secretsExist bool

	planning bool
	planErr  error
	plan     install.InstallPlan
	summary  viewport.Model

	stream   installStream
	statuses []stepStatus
	current  int
	log      []string
	logView  viewport.Model
	follow   bool
	ack      chan struct{}
	awaitAck bool
	running  bool
	finished bool
	runErr   error
}

func newInstall(svc *services) *Install {
	input := textinput.New()
	input.EchoMode = textinput.EchoNone
	return &Install{
		ctx:         svc.ctx,
		deps:        svc.Install,
		keys:        svc.keys,
		th:          svc.th,
		draft:       install.Draft{Profile: manifest.ProfileMinimal},
		secretInput: input,
		summary:     viewport.New(),
		logView:     viewport.New(),
		follow:      true,
	}
}

func (s *Install) Title() string  { return "Install" }
func (s *Install) Enter() tea.Cmd { return nil }

// Keys lists the current page's bindings. Pages after the first claim esc to go back; the run page claims it only once finished.
func (s *Install) Keys() []key.Binding {
	switch s.page {
	case pageProfile:
		return []key.Binding{s.keys.Up, s.keys.Down, s.keys.Confirm}
	case pageHardware, pageSelection:
		return []key.Binding{s.keys.Up, s.keys.Down, s.keys.Toggle, s.keys.Confirm, s.keys.Back}
	case pageSecrets:
		keys := []key.Binding{s.keys.Confirm, s.keys.Back}
		if s.secretPrompting() {
			keys = append(keys, s.keys.SkipSecrets)
		}
		return keys
	case pageSummary:
		return []key.Binding{s.keys.Up, s.keys.Down, s.keys.Confirm, s.keys.Back}
	}
	keys := []key.Binding{s.keys.Up, s.keys.Down}
	if s.awaitAck {
		keys = append(keys, s.keys.Confirm)
	}
	if s.finished {
		keys = append(keys, s.keys.Back)
	}
	return keys
}

// CapturesText reports whether printable keys belong to the hidden input, so the shell must not act on them.
func (s *Install) CapturesText() bool {
	return s.page == pageSecrets && s.secretPrompting()
}

func (s *Install) secretPrompting() bool {
	return !s.secretsExist && s.secretIdx < len(s.deps.SecretNames)
}

// Cancel stops the running plan or install, if any.
func (s *Install) Cancel() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *Install) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case gpuDetectedMsg:
		s.hwLoading, s.hwLoaded, s.detected = false, true, msg.detected
		s.draft.Drivers = msg.ids
	case installPlanMsg:
		if msg.gen == s.gen {
			s.planning, s.plan, s.planErr = false, msg.plan, msg.err
			s.showSummary()
		}
	case installStartedMsg:
		if msg.gen == s.gen {
			s.stream = msg.stream
			return s, waitInstall(msg.gen, msg.stream)
		}
	case installEventMsg:
		if msg.gen == s.gen {
			s.handleEvent(msg.event)
			return s, waitInstall(msg.gen, s.stream)
		}
	case installDoneMsg:
		if msg.gen == s.gen {
			s.finishRun(msg.result)
		}
	case tea.KeyPressMsg:
		return s, s.handleKey(msg)
	}
	return s, nil
}

func (s *Install) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch s.page {
	case pageProfile:
		return s.profileKey(msg)
	case pageHardware:
		return s.hardwareKey(msg)
	case pageSelection:
		return s.selectionKey(msg)
	case pageSecrets:
		return s.secretsKey(msg)
	case pageSummary:
		return s.summaryKey(msg)
	}
	return s.runKey(msg)
}

// moveCursor applies up and down over n rows and reports whether the key was a move.
func (s *Install) moveCursor(msg tea.KeyPressMsg, n int) bool {
	switch {
	case key.Matches(msg, s.keys.Up):
		s.cursor = max(s.cursor-1, 0)
	case key.Matches(msg, s.keys.Down):
		s.cursor = max(min(s.cursor+1, n-1), 0)
	default:
		return false
	}
	return true
}

// skipsPage reports pages the flow passes through: no custom selection, no secret names.
func (s *Install) skipsPage(p installPage) bool {
	switch p {
	case pageSelection:
		return s.draft.Profile != install.ProfileCustom
	case pageSecrets:
		return len(s.deps.SecretNames) == 0
	}
	return false
}

// step moves one page forward (+1) or back (-1) over skipped pages. Entering the secrets page resets the secrets:
// retyping is safer than keeping values in memory.
func (s *Install) step(direction int) tea.Cmd {
	p := s.page
	for {
		p += installPage(direction)
		if p < pageProfile || p > pageSummary {
			return nil
		}
		if !s.skipsPage(p) {
			break
		}
	}
	s.page = p
	return s.enterPage()
}

func (s *Install) enterPage() tea.Cmd {
	s.cursor = 0
	switch s.page {
	case pageProfile:
		s.cursor = max(slices.Index(profileChoices, s.draft.Profile), 0)
	case pageHardware:
		if !s.hwLoaded && !s.hwLoading {
			s.hwLoading = true
			return detectGPUs(s.deps.DetectGPUs, s.deps.Drivers)
		}
	case pageSelection:
		s.seedSelection()
	case pageSecrets:
		s.enterSecrets()
	case pageSummary:
		return s.startPlan()
	}
	return nil
}

// startRun is reachable only from an explicit confirm on the summary.
func (s *Install) startRun() tea.Cmd {
	s.Cancel()
	s.gen++
	s.page = pageRun
	s.statuses = make([]stepStatus, len(s.plan.Steps))
	s.current, s.log, s.follow = 0, nil, true
	s.running, s.finished, s.awaitAck, s.runErr = true, false, false, nil
	s.logView.SetContentLines(nil)
	ctx, cancel := context.WithCancel(s.ctx)
	s.cancel = cancel
	ack := make(chan struct{}, 1)
	s.ack = ack
	gen, run, installPlan := s.gen, s.deps.Run, s.plan
	// The plan snapshot carries the values Run needs; the draft must not keep them.
	s.draft.Secrets = nil
	runDone := make(chan struct{})
	s.runDone = runDone
	return func() tea.Msg {
		events := make(chan plan.Event)
		result := make(chan install.InstallResult, 1)
		go func() {
			defer close(runDone)
			result <- run(ctx, installPlan, events, ack)
		}()
		return installStartedMsg{gen: gen, stream: installStream{events: events, result: result}}
	}
}

// waitInstall reads one event and is re-issued by Update until Run closes the channel, as for the sync apply.
func waitInstall(gen int, st installStream) tea.Cmd {
	return func() tea.Msg {
		if ev, ok := <-st.events; ok {
			return installEventMsg{gen: gen, event: ev}
		}
		return installDoneMsg{gen: gen, result: <-st.result}
	}
}

func (s *Install) handleEvent(ev plan.Event) {
	if ev.Kind == plan.EventLine {
		s.appendLog(ev.Line)
		return
	}
	if ev.Index < 0 || ev.Index >= len(s.statuses) {
		return
	}
	s.current = ev.Index
	switch ev.Kind {
	case plan.EventStarted, plan.EventAwaitAck:
		s.statuses[ev.Index] = statusRunning
		s.awaitAck = ev.Kind == plan.EventAwaitAck
	case plan.EventDone:
		s.statuses[ev.Index] = statusDone
	case plan.EventSkipped:
		s.statuses[ev.Index] = statusSkipped
	case plan.EventFailed:
		s.statuses[ev.Index] = statusFailed
		s.runErr = ev.Err
	}
}

func (s *Install) appendLog(line string) {
	s.log = append(s.log, sanitizeLine(line))
	if len(s.log) > logLineCap {
		s.log = s.log[len(s.log)-logLineCap:]
	}
	s.logView.SetContentLines(s.log)
}

// finishRun does not depend on trailing events: after a cancel they may have been dropped.
func (s *Install) finishRun(result install.InstallResult) {
	s.running, s.finished, s.awaitAck = false, true, false
	if s.runErr == nil {
		s.runErr = result.Err
	}
	s.cancel = nil
	s.plan = withoutSecretValues(s.plan)
}

// withoutSecretValues keeps the step list for the run page but drops the secret values.
func withoutSecretValues(installPlan install.InstallPlan) install.InstallPlan {
	installPlan.Steps = slices.Clone(installPlan.Steps)
	for i, ps := range installPlan.Steps {
		if w, ok := ps.Step.(plan.WriteSecretsStep); ok {
			w.Entries = slices.Clone(w.Entries)
			for j := range w.Entries {
				w.Entries[j].Value = ""
			}
			installPlan.Steps[i].Step = w
		}
	}
	return installPlan
}

// WaitRun blocks until the install goroutine has returned (log persisted, child reaped) or timeout passes.
func (s *Install) WaitRun(timeout time.Duration) {
	if s.runDone == nil {
		return
	}
	select {
	case <-s.runDone:
	case <-time.After(timeout):
	}
}
