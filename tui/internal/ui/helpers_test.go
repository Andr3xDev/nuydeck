package ui

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/references"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

var (
	keyTab   = tea.KeyPressMsg{Code: tea.KeyTab}
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyRight = tea.KeyPressMsg{Code: tea.KeyRight}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyUp    = tea.KeyPressMsg{Code: tea.KeyUp}
	keyCtrlC = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
)

func keyPress(code rune) tea.KeyPressMsg {
	if code == tea.KeySpace {
		return tea.KeyPressMsg{Code: code, Text: " "}
	}
	if code == tea.KeyEnter {
		return tea.KeyPressMsg{Code: code}
	}
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

func specialKey(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

// feed runs cmd and every command it yields, delivering each message to s; run requests are collected, not started.
func feed(s screen, cmd tea.Cmd) (runs []runRequest) {
	var walk func(tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch m := c().(type) {
		case nil:
		case tea.BatchMsg:
			for _, sub := range m {
				walk(sub)
			}
		case runRequest:
			runs = append(runs, m)
		default:
			_, next := s.Update(m)
			walk(next)
		}
	}
	walk(cmd)
	return runs
}

// send presses keys in order on the screen and returns every run request they led to.
func send(s screen, keys ...tea.KeyPressMsg) (runs []runRequest) {
	for _, k := range keys {
		_, cmd := s.Update(k)
		runs = append(runs, feed(s, cmd)...)
	}
	return runs
}

func viewOf(s screen) string { return ansi.Strip(s.View(120, 40)) }

func containsAny(text string, options ...string) bool {
	text = strings.ToLower(text)
	return slices.ContainsFunc(options, func(o string) bool { return strings.Contains(text, strings.ToLower(o)) })
}

func downTimes(n int) []tea.KeyPressMsg {
	keys := make([]tea.KeyPressMsg, n)
	for i := range keys {
		keys[i] = keyDown
	}
	return keys
}

type fakeSyncDeps struct {
	plan          cfgsync.SyncPlan
	needsBackup   bool
	progressPaths []string
	blockUntilCtx bool
	result        cfgsync.ApplyResult
	opts          cfgsync.ApplyOptions
	applyCalls    atomic.Int32
}

func (f *fakeSyncDeps) deps() Deps {
	return Deps{PlanSync: f.PlanSync, NeedsBackup: f.NeedsBackup, ApplySync: f.Apply}
}

func (f *fakeSyncDeps) PlanSync(context.Context, plan.Direction, []manifest.ResolvedEntry) (cfgsync.SyncPlan, error) {
	return f.plan, nil
}

func (f *fakeSyncDeps) NeedsBackup(cfgsync.SyncPlan, cfgsync.Selection) bool { return f.needsBackup }

func (f *fakeSyncDeps) Apply(ctx context.Context, _ cfgsync.SyncPlan, _ cfgsync.Selection, opts cfgsync.ApplyOptions, progress func(plan.StepProgress)) cfgsync.ApplyResult {
	f.applyCalls.Add(1)
	f.opts = opts
	for _, p := range f.progressPaths {
		progress(plan.StepProgress{Path: p})
	}
	if f.blockUntilCtx {
		<-ctx.Done()
	}
	return f.result
}

func selectedPlan() cfgsync.SyncPlan {
	item := planItem("a", false)
	return cfgsync.SyncPlan{Items: []cfgsync.PlanItem{item}, Selection: cfgsync.Selection{item.Key(): true}}
}

func planItem(path string, blocked bool) cfgsync.PlanItem {
	return cfgsync.PlanItem{Change: plan.Change{Entry: manifest.Entry{Root: "home", Path: path}, Type: plan.Update}, Blocked: blocked}
}

func newPreview(f *fakeSyncDeps, dryRun bool) *Sync {
	svc := testServices(f.deps())
	svc.dryRun = dryRun
	s := newSync(svc)
	runCmds(s, s.startPlan(plan.Push, nil))
	return s
}

type fakeTerminal struct {
	cmds []*exec.Cmd
	err  error
}

func (f *fakeTerminal) Run(_ context.Context, cmd *exec.Cmd) error {
	f.cmds = append(f.cmds, cmd)
	return f.err
}

type stateRead struct {
	scope plan.Scope
	units []string
}

// fakeServices serves a fixed catalog; Plan is the real services.PlanAction so risk and not-found rules are the domain's.
type fakeServices struct {
	catalog []services.Entry
	states  map[string]services.UnitState
	busErr  error
	runErr  error

	reads   []stateRead
	planned []services.Request
	ran     []plan.Plan
}

func (f *fakeServices) deps() ServicesDeps {
	return ServicesDeps{Catalog: f.catalog, State: f.State, Plan: f.Plan}
}

func (f *fakeServices) State(_ context.Context, scope plan.Scope, units []string) (map[string]services.UnitState, error) {
	f.reads = append(f.reads, stateRead{scope: scope, units: units})
	if scope == plan.ScopeUser && f.busErr != nil {
		return nil, f.busErr
	}
	out := map[string]services.UnitState{}
	for _, u := range units {
		out[u] = f.states[u]
	}
	return out, nil
}

func (f *fakeServices) Plan(_ context.Context, req services.Request, state services.UnitState) (plan.Plan, error) {
	f.planned = append(f.planned, req)
	return services.PlanAction(f.catalog, req, state, true)
}

func (f *fakeServices) Run(_ context.Context, p plan.Plan, _ func(plan.Event)) error {
	f.ran = append(f.ran, p)
	return f.runErr
}

// The system units come first and in this order: that is the order of the Services list.
const (
	unitNetworkManager = "NetworkManager.service"
	unitBluetooth      = "bluetooth.service"
	unitGhost          = "ghost.service"
	unitMasked         = "masked.service"
	unitBroken         = "broken.service"
	unitClipboardTimer = "lucyna-clipboard-wipe.timer"
)

const (
	rowNetworkManager = iota
	rowBluetooth
	rowGhost
	rowMasked
	rowBroken
	rowClipboardTimer
)

func newFakeServices() *fakeServices {
	return &fakeServices{
		catalog: []services.Entry{
			{Unit: unitNetworkManager, Scope: plan.ScopeSystem, Source: services.SourcePackage, Critical: true},
			{Unit: unitBluetooth, Scope: plan.ScopeSystem, Source: services.SourcePackage},
			{Unit: unitGhost, Scope: plan.ScopeSystem, Source: services.SourcePackage},
			{Unit: unitMasked, Scope: plan.ScopeSystem, Source: services.SourcePackage},
			{Unit: unitBroken, Scope: plan.ScopeSystem, Source: services.SourcePackage},
			{Unit: unitClipboardTimer, Scope: plan.ScopeUser, Source: services.SourceCustom},
		},
		states: map[string]services.UnitState{
			unitNetworkManager: {Found: true, Enabled: services.EnabledEnabled, Active: services.ActiveActive},
			unitBluetooth:      {Found: true, Enabled: services.EnabledDisabled, Active: services.ActiveInactive},
			unitMasked:         {Found: true, Enabled: services.EnabledOther, Active: services.ActiveInactive, Masked: true},
			unitBroken:         {Found: true, Enabled: services.EnabledEnabled, Active: services.ActiveFailed},
			unitClipboardTimer: {Found: true, Enabled: services.EnabledEnabled, Active: services.ActiveActive},
		},
	}
}

func (f *fakeServices) entry(unit string) services.Entry {
	return f.catalog[slices.IndexFunc(f.catalog, func(e services.Entry) bool { return e.Unit == unit })]
}

func hyprEntry() manifest.ResolvedEntry {
	return manifest.ResolvedEntry{
		Entry:    manifest.Entry{Root: manifest.RootConfig, Path: "hypr"},
		RepoPath: "/repo/config/hypr",
		DestPath: "/home/user/.config/hypr",
	}
}

// fakeEnv holds the env file copies by path and records every read and edit.
type fakeEnv struct {
	files       map[string]cfgsync.Side
	reads       []string
	edits       []string
	editPreview string
	editErr     error
	// afterEdit replaces the content of the edited file, as a saved editor session would.
	afterEdit string
	ran       []plan.Plan
	runErr    error
}

func (f *fakeEnv) Run(_ context.Context, p plan.Plan, _ func(plan.Event)) error {
	f.ran = append(f.ran, p)
	return f.runErr
}

func fileSide(content string) cfgsync.Side {
	return cfgsync.Side{Exists: true, Kind: plan.KindFile, Content: []byte(content), Size: int64(len(content))}
}

func (f *fakeEnv) ReadSide(path string) (cfgsync.Side, error) {
	f.reads = append(f.reads, path)
	return f.files[path], nil
}

func (f *fakeEnv) Edit(_ context.Context, path string) (string, error) {
	f.edits = append(f.edits, path)
	if f.afterEdit != "" {
		f.files[path] = fileSide(f.afterEdit)
	}
	return f.editPreview, f.editErr
}

func newFakeEnv(repoText, liveText string) *fakeEnv {
	repo, live := "/repo/config/hypr/configs/env_variables.lua", "/home/user/.config/hypr/configs/env_variables.lua"
	return &fakeEnv{files: map[string]cfgsync.Side{repo: fileSide(repoText), live: fileSide(liveText)}}
}

const (
	envRepoText = "-- cursor\nhl.env(\"THEME\", \"repo-value\")\nlocal x = 1\n-- hl.env(\"COMMENTED\", \"no\")\n"
	envLiveText = "hl.env(\"THEME\", \"live-value\")\n"
)

func (f *fakeEnv) deps(base Deps) Deps {
	base.Entries = []manifest.ResolvedEntry{hyprEntry()}
	base.ReadSide = f.ReadSide
	base.Env = EnvDeps{Edit: f.Edit}
	return base
}

// fakeReferences lists fixed entries with a status by name and records the links opened.
type fakeReferences struct {
	refs    []references.Reference
	status  map[string]references.Status
	opened  []string
	openErr error
}

func newFakeReferences() *fakeReferences {
	return &fakeReferences{
		refs: []references.Reference{
			{Name: "arch-linux", Description: "Distribution home", URL: "https://archlinux.org"},
			{Name: "nuytek-theme", Description: "GTK theme", URL: "https://github.com/Andr3xDev/nuytek-theme"},
			{Name: "mystery", Description: "No probe", URL: "https://example.org"},
		},
		status: map[string]references.Status{"arch-linux": references.StatusPresent, "nuytek-theme": references.StatusAbsent},
	}
}

func (f *fakeReferences) deps() ReferencesDeps {
	return ReferencesDeps{
		Catalog: f.refs,
		Status:  func(ref references.Reference) references.Status { return f.status[ref.Name] },
		Open:    func(_ context.Context, url string) error { f.opened = append(f.opened, url); return f.openErr },
	}
}

func fullDeps() Deps {
	return newFakeEnv(envRepoText, envLiveText).deps(Deps{
		Services:   newFakeServices().deps(),
		References: newFakeReferences().deps(),
	})
}
