package ui

import (
	"context"
	"errors"
	"slices"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

var errInstallUnavailable = errors.New("install is not available")

type installPage int

const (
	pageProfile installPage = iota
	pageHardware
	pageSelection
	pageSecrets
	pageSummary
)

var profileChoices = []string{manifest.ProfileMinimal, manifest.ProfileFull, install.ProfileCustom}

type gpuDetectedMsg struct {
	ids      []string
	detected bool
}

type installPlanMsg struct {
	gen  int
	plan install.InstallPlan
	err  error
}

type selectionRow struct {
	group string
	name  string // empty for a group header
}

// Install is the guided install wizard: profile, hardware, selection, secrets, summary. The run belongs to the shell.
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
	// started is true from the confirmed summary until the run pane closes, when the summary plans again.
	started bool

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
}

func newInstall(svc *services) *Install {
	input := textinput.New()
	input.EchoMode = textinput.EchoNone
	summary := viewport.New()
	summary.SoftWrap = true
	return &Install{
		ctx:         svc.ctx,
		deps:        svc.Install,
		keys:        svc.keys,
		th:          svc.th,
		draft:       install.Draft{Profile: manifest.ProfileMinimal},
		secretInput: input,
		summary:     summary,
	}
}

func (s *Install) Title() string  { return "Install" }
func (s *Install) Enter() tea.Cmd { return nil }

// Keys lists the current page's bindings. Pages after the first claim esc to go back one step.
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
	}
	return []key.Binding{s.keys.Up, s.keys.Down, s.keys.Confirm, s.keys.Back}
}

// CapturesText reports whether printable keys belong to the hidden input, so the shell must not act on them.
func (s *Install) CapturesText() bool {
	return s.page == pageSecrets && s.secretPrompting()
}

func (s *Install) secretPrompting() bool {
	return !s.secretsExist && s.secretIdx < len(s.deps.SecretNames)
}

// Leave forgets the typed secret values once the owner moves off Install or quits.
func (s *Install) Leave() {
	s.draft.Secrets = nil
	s.plan = withoutSecretValues(s.plan)
	s.secretInput.Reset()
}

// Cancel stops the running plan or install, if any.
func (s *Install) Cancel() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// Notices are the plan's notices while the summary shows a plan.
func (s *Install) Notices() []string {
	if s.page != pageSummary || s.planning || s.planErr != nil {
		return nil
	}
	return s.plan.Notices
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
	case runClosed:
		if s.started {
			s.started = false
			return s, s.startPlan()
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
	}
	return s.summaryKey(msg)
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
	s.started = true
	run, installPlan := s.deps.Run, s.plan
	// The draft keeps the values for a retry until Leave; the screen's plan never shows them.
	s.plan = withoutSecretValues(s.plan)
	steps := make([]string, len(installPlan.Steps))
	for i, ps := range installPlan.Steps {
		steps[i] = stepLabel(ps.Step)
	}
	ack := make(chan struct{}, 1)
	req := runRequest{title: "Install", ack: ack, steps: steps, footer: installFooter, work: func(ctx context.Context, emit func(plan.Event)) runEnd {
		events := make(chan plan.Event)
		result := make(chan install.InstallResult, 1)
		go func() { result <- run(ctx, installPlan, events, ack) }()
		for ev := range events {
			emit(ev)
		}
		return runEnd{err: (<-result).Err}
	}}
	return func() tea.Msg { return req }
}

// withoutSecretValues keeps the step list for the summary but drops the secret values.
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
