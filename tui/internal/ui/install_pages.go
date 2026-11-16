package ui

import (
	"context"
	"slices"
	"sort"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func detectGPUs(detect func(context.Context) ([]install.PCIDevice, error), drivers []manifest.Driver) tea.Cmd {
	return func() tea.Msg {
		var devices []install.PCIDevice
		if detect != nil {
			// A failed probe leaves devices empty, which proposes every driver with detected=false.
			devices, _ = detect(context.Background())
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
	if s.deps.PathKind != nil {
		kind, err := s.deps.PathKind(s.deps.SecretsPath)
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
	s.Cancel()
	s.gen++
	s.plan, s.planErr, s.planning = install.InstallPlan{}, nil, true
	s.showSummary()
	if s.deps.Plan == nil {
		s.planning, s.planErr = false, errInstallUnavailable
		return nil
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.cancel = cancel
	gen, planInstall, draft := s.gen, s.deps.Plan, s.draftSnapshot()
	return func() tea.Msg {
		plan, err := planInstall(ctx, draft)
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
		s.Cancel()
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
