package ui

import (
	"context"
	"errors"
	"slices"
	"sort"
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

// Installer is the install use-case surface the screen needs; *install.Installer implements it.
type Installer interface {
	Plan(ctx context.Context, d install.Draft) (install.InstallPlan, error)
	Run(ctx context.Context, p install.InstallPlan, events chan<- plan.InstallEvent, ack <-chan struct{}) install.InstallResult
}

// GPUDetector is the read-only GPU probe of the hardware page.
type GPUDetector interface {
	DetectGPUs(ctx context.Context) ([]install.PCIDevice, error)
}

// PathKinder checks whether a path exists without opening it.
type PathKinder interface {
	PathKind(path string) (plan.NodeKind, error)
}

// InstallDeps is what the Install section needs besides the shell.
type InstallDeps struct {
	Installer Installer
	// Preview is always a dry-run recorder; the summary renders each planned step through it.
	Preview     plan.Runner
	GPUs        GPUDetector
	Paths       PathKinder
	SecretsPath string
	SecretNames []string
	Catalog     []manifest.Package
	Drivers     []manifest.Driver
}

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
	events <-chan plan.InstallEvent
	result <-chan install.InstallResult
}

type installStartedMsg struct {
	gen    int
	stream installStream
}

type installEventMsg struct {
	gen   int
	event plan.InstallEvent
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
	deps    InstallDeps
	keys    KeyMap
	palette Palette

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

func newInstall(deps InstallDeps, keys KeyMap, palette Palette) *Install {
	input := textinput.New()
	input.EchoMode = textinput.EchoNone
	return &Install{
		deps:        deps,
		keys:        keys,
		palette:     palette,
		draft:       install.Draft{Profile: manifest.ProfileMinimal},
		secretInput: input,
		summary:     viewport.New(),
		logView:     viewport.New(),
		follow:      true,
	}
}

// Modal reports whether q and esc must reach the screen: every page after the first goes back with esc.
func (s *Install) Modal() bool { return s.page != pageProfile }

// capturesText reports whether printable keys belong to the hidden input, so the shell must not act on them.
func (s *Install) capturesText() bool {
	return s.page == pageSecrets && s.secretPrompting()
}

func (s *Install) secretPrompting() bool {
	return !s.secretsExist && s.secretIdx < len(s.deps.SecretNames)
}

// cancelWork stops the running plan or install, if any.
func (s *Install) cancelWork() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *Install) Update(msg tea.Msg) (Screen, tea.Cmd) {
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
			s.applyEvent(msg.event)
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

func (s *Install) dismiss(msg tea.KeyPressMsg) bool {
	return key.Matches(msg, s.keys.Back) || msg.String() == "q"
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
			return detectGPUs(s.deps.GPUs, s.deps.Drivers)
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

func detectGPUs(gpus GPUDetector, drivers []manifest.Driver) tea.Cmd {
	return func() tea.Msg {
		var devices []install.PCIDevice
		if gpus != nil {
			// A failed probe leaves devices empty, which proposes every driver with detected=false.
			devices, _ = gpus.DetectGPUs(context.Background())
		}
		ids, detected := install.ProposeDrivers(devices, drivers)
		return gpuDetectedMsg{ids: ids, detected: detected}
	}
}

func (s *Install) profileKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case s.moveCursor(msg, len(profileChoices)):
	case key.Matches(msg, s.keys.Confirm):
		s.draft.Profile = profileChoices[s.cursor]
		return s.step(1)
	}
	return nil
}

func (s *Install) hardwareKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case s.dismiss(msg):
		return s.step(-1)
	case s.hwLoading:
	case s.moveCursor(msg, len(s.deps.Drivers)+1):
	case key.Matches(msg, s.keys.Toggle):
		if s.cursor == len(s.deps.Drivers) {
			s.draft.SkipGPUEnvLink = !s.draft.SkipGPUEnvLink
		} else {
			s.toggleDriver(s.deps.Drivers[s.cursor].ID)
		}
	case key.Matches(msg, s.keys.Confirm):
		return s.step(1)
	}
	return nil
}

func (s *Install) toggleDriver(id string) {
	if i := slices.Index(s.draft.Drivers, id); i >= 0 {
		s.draft.Drivers = slices.Delete(slices.Clone(s.draft.Drivers), i, i+1)
		return
	}
	s.draft.Drivers = append(slices.Clone(s.draft.Drivers), id)
}

// seedSelection builds the group rows and, once, preselects the minimal profile so custom starts from something usable.
func (s *Install) seedSelection() {
	if !s.seeded {
		s.seeded = true
		s.selected = install.DefaultSelection(s.draft.Profile, s.deps.Catalog)
	}
	pkgs := slices.Clone(s.deps.Catalog)
	sort.SliceStable(pkgs, func(i, j int) bool {
		if pkgs[i].Group != pkgs[j].Group {
			return pkgs[i].Group < pkgs[j].Group
		}
		return pkgs[i].Name < pkgs[j].Name
	})
	s.rows = s.rows[:0]
	for i, p := range pkgs {
		if i == 0 || p.Group != pkgs[i-1].Group {
			s.rows = append(s.rows, selectionRow{group: p.Group})
		}
		s.rows = append(s.rows, selectionRow{group: p.Group, name: p.Name})
	}
}

func (s *Install) selectionKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case s.dismiss(msg):
		return s.step(-1)
	case s.moveCursor(msg, len(s.rows)):
	case key.Matches(msg, s.keys.Toggle) && s.cursor < len(s.rows):
		s.toggleRow(s.rows[s.cursor])
	case key.Matches(msg, s.keys.Confirm):
		return s.step(1)
	}
	return nil
}

func (s *Install) toggleRow(row selectionRow) {
	if row.name != "" {
		s.selected[row.name] = !s.selected[row.name]
		return
	}
	names := s.groupNames(row.group)
	all := !slices.ContainsFunc(names, func(n string) bool { return !s.selected[n] })
	for _, n := range names {
		s.selected[n] = !all
	}
}

func (s *Install) groupNames(group string) []string {
	var names []string
	for _, p := range s.deps.Catalog {
		if p.Group == group {
			names = append(names, p.Name)
		}
	}
	return names
}

func (s *Install) enterSecrets() {
	s.draft.Secrets, s.draft.SkipSecrets = nil, false
	s.secretIdx = 0
	s.secretInput.SetValue("")
	s.secretsExist = false
	if s.deps.Paths != nil {
		kind, err := s.deps.Paths.PathKind(s.deps.SecretsPath)
		s.secretsExist = err == nil && kind != plan.KindNone
	}
	// The blink command is dropped on purpose: EchoNone shows no cursor.
	_ = s.secretInput.Focus()
}

func (s *Install) secretsKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case s.dismiss(msg) && (!s.secretPrompting() || key.Matches(msg, s.keys.Back)):
		return s.step(-1)
	case !s.secretPrompting():
		if key.Matches(msg, s.keys.Confirm) {
			return s.step(1)
		}
		return nil
	case msg.String() == "ctrl+s":
		s.draft.Secrets, s.draft.SkipSecrets = nil, true
		s.secretInput.SetValue("")
		return s.step(1)
	case key.Matches(msg, s.keys.Confirm):
		return s.submitSecret()
	}
	s.secretInput, _ = s.secretInput.Update(msg)
	return nil
}

// submitSecret stores the typed value (an empty one omits the variable) and moves to the next name.
func (s *Install) submitSecret() tea.Cmd {
	if value := s.secretInput.Value(); value != "" {
		s.draft.Secrets = append(s.draft.Secrets, plan.SecretEntry{Name: s.deps.SecretNames[s.secretIdx], Value: plan.Secret(value)})
	}
	s.secretInput.SetValue("")
	s.secretIdx++
	if s.secretIdx < len(s.deps.SecretNames) {
		return nil
	}
	return s.step(1)
}

// draftSnapshot is what the plan sees; slices are copied because the plan runs on another goroutine.
func (s *Install) draftSnapshot() install.Draft {
	d := s.draft
	d.Drivers = slices.Clone(d.Drivers)
	d.Secrets = slices.Clone(d.Secrets)
	d.Packages = nil
	if d.Profile == install.ProfileCustom {
		for _, p := range s.deps.Catalog {
			if s.selected[p.Name] {
				d.Packages = append(d.Packages, p.Name)
			}
		}
	}
	return d
}

func (s *Install) startPlan() tea.Cmd {
	s.cancelWork()
	s.gen++
	s.plan, s.planErr, s.planning = install.InstallPlan{}, nil, true
	s.showSummary()
	if s.deps.Installer == nil {
		s.planning, s.planErr = false, errInstallUnavailable
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	gen, installer, draft := s.gen, s.deps.Installer, s.draftSnapshot()
	return func() tea.Msg {
		plan, err := installer.Plan(ctx, draft)
		return installPlanMsg{gen: gen, plan: plan, err: err}
	}
}

// canConfirm is false while planning, after a plan error and whenever the plan has blocking errors.
func (s *Install) canConfirm() bool {
	return !s.planning && s.planErr == nil && len(s.plan.Blocking) == 0 && len(s.plan.Steps) > 0
}

func (s *Install) summaryKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case s.dismiss(msg):
		s.cancelWork()
		return s.step(-1)
	case key.Matches(msg, s.keys.Up):
		s.summary.ScrollUp(1)
	case key.Matches(msg, s.keys.Down):
		s.summary.ScrollDown(1)
	case key.Matches(msg, s.keys.Confirm) && s.canConfirm():
		return s.startRun()
	}
	return nil
}

// startRun is reachable only from an explicit confirm on the summary.
func (s *Install) startRun() tea.Cmd {
	s.cancelWork()
	s.gen++
	s.page = pageRun
	s.statuses = make([]stepStatus, len(s.plan.Steps))
	s.current, s.log, s.follow = 0, nil, true
	s.running, s.finished, s.awaitAck, s.runErr = true, false, false, nil
	s.logView.SetContentLines(nil)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	ack := make(chan struct{}, 1)
	s.ack = ack
	gen, installer, installPlan := s.gen, s.deps.Installer, s.plan
	// The plan snapshot carries the values Run needs; the draft must not keep them.
	s.draft.Secrets = nil
	runDone := make(chan struct{})
	s.runDone = runDone
	return func() tea.Msg {
		events := make(chan plan.InstallEvent)
		result := make(chan install.InstallResult, 1)
		go func() {
			defer close(runDone)
			result <- installer.Run(ctx, installPlan, events, ack)
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

func (s *Install) applyEvent(ev plan.InstallEvent) {
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
	s.log = append(s.log, line)
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

func (s *Install) runKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, s.keys.Up):
		s.logView.ScrollUp(1)
		s.follow = false
	case key.Matches(msg, s.keys.Down):
		s.logView.ScrollDown(1)
		s.follow = s.logView.AtBottom()
	case s.awaitAck:
		if key.Matches(msg, s.keys.Confirm) {
			s.awaitAck = false
			select {
			case s.ack <- struct{}{}:
			default:
			}
		}
	case s.finished && key.Matches(msg, s.keys.Back):
		return s.step(-1)
	case s.finished && msg.String() == "q":
		return tea.Quit
	}
	return nil
}
