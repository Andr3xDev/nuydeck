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
	case s.moveBy(msg, len(profileChoices)):
	case key.Matches(msg, s.keys.Confirm):
		s.draft.Profile = profileChoices[s.cursor]
		return s.step(1)
	}
	return nil
}

func (s *Install) hardwareKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, s.keys.Back):
		return s.step(-1)
	case s.hwLoading:
	case s.moveBy(msg, len(s.deps.Drivers)+1):
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

// buildRows lists the catalog by group, then the packages installed as dependencies under a last group of their own.
func buildRows(catalog []manifest.Package, dependencies map[string]bool) []selectionRow {
	pkgs := slices.Clone(catalog)
	sort.SliceStable(pkgs, func(i, j int) bool {
		if pkgs[i].Group != pkgs[j].Group {
			return pkgs[i].Group < pkgs[j].Group
		}
		return pkgs[i].Name < pkgs[j].Name
	})
	var rows []selectionRow
	var dependencyRows []selectionRow
	for _, p := range pkgs {
		if dependencies[p.Name] {
			dependencyRows = append(dependencyRows, selectionRow{group: dependenciesHeader, name: p.Name})
			continue
		}
		if n := len(rows); n == 0 || rows[n-1].group != p.Group {
			rows = append(rows, selectionRow{group: p.Group})
		}
		rows = append(rows, selectionRow{group: p.Group, name: p.Name})
	}
	if len(dependencyRows) > 0 {
		sort.SliceStable(dependencyRows, func(i, j int) bool { return dependencyRows[i].name < dependencyRows[j].name })
		rows = append(rows, selectionRow{group: dependenciesHeader})
		rows = append(rows, dependencyRows...)
	}
	return withHeaderNames(rows)
}

// withHeaderNames fills each header's names with the packages under it.
func withHeaderNames(rows []selectionRow) []selectionRow {
	header := -1
	for i, r := range rows {
		if r.name == "" {
			header = i
			continue
		}
		rows[header].names = append(rows[header].names, r.name)
	}
	return rows
}

// seedSelection builds the group rows and, once, preselects the minimal profile so custom starts from something usable.
func (s *Install) seedSelection() {
	if !s.seeded {
		s.seeded = true
		s.selected = install.DefaultSelection(s.draft.Profile, s.deps.Catalog)
	}
	s.rows = buildRows(s.deps.Catalog, s.dependencies)
}

// loadDependencies probes once per wizard; a failed probe leaves the group out.
func loadDependencies(probe func(context.Context) (map[string]bool, error)) tea.Cmd {
	return func() tea.Msg {
		set, _ := probe(context.Background())
		return dependenciesMsg{set: set}
	}
}

// applyDependencies rebuilds the rows around the late probe result, keeping the cursor on the same row.
func (s *Install) applyDependencies(set map[string]bool) {
	s.depsLoaded, s.dependencies = true, set
	var current selectionRow
	if s.cursor < len(s.rows) {
		current = s.rows[s.cursor]
	}
	s.rows = buildRows(s.deps.Catalog, set)
	for i, r := range s.rows {
		if r.name == current.name && (r.name != "" || r.group == current.group) {
			s.cursor = i
			return
		}
	}
	s.cursor = max(min(s.cursor, len(s.rows)-1), 0)
}

func (s *Install) selectionKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, s.keys.Back):
		return s.step(-1)
	case s.moveBy(msg, len(s.rows)):
	case key.Matches(msg, s.keys.Toggle) && s.cursor < len(s.rows):
		s.toggleRow(s.rows[s.cursor])
	case key.Matches(msg, s.keys.All):
		s.toggleAll()
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
	s.setAll(row.names)
}

func (s *Install) toggleAll() {
	var names []string
	for _, r := range s.rows {
		if r.name != "" {
			names = append(names, r.name)
		}
	}
	s.setAll(names)
}

// setAll selects the names, or clears them when they are all selected already.
func (s *Install) setAll(names []string) {
	all := !slices.ContainsFunc(names, func(n string) bool { return !s.selected[n] })
	for _, n := range names {
		s.selected[n] = !all
	}
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
	case key.Matches(msg, s.keys.Back):
		return s.step(-1)
	case !s.secretPrompting():
		if key.Matches(msg, s.keys.Confirm) {
			return s.step(1)
		}
		return nil
	case key.Matches(msg, s.keys.SkipSecrets):
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
	case key.Matches(msg, s.keys.Back):
		s.Cancel()
		return s.step(-1)
	case s.keys.step(msg) < 0:
		s.summary.ScrollUp(-s.keys.step(msg))
	case s.keys.step(msg) > 0:
		s.summary.ScrollDown(s.keys.step(msg))
	case key.Matches(msg, s.keys.Confirm) && s.canConfirm() && !s.started:
		return s.startRun()
	}
	return nil
}
