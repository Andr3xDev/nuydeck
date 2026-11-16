package ui

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// PackageManager is the packages use-case surface the screen needs; *packages.Packages implements it.
type PackageManager interface {
	ListUnregistered(ctx context.Context) ([]string, error)
	AddPackage(ctx context.Context, pkg manifest.Package) (plan.StepResult, error)
	IgnorePackage(ctx context.Context, name string) plan.StepResult
}

type unregisteredMsg struct {
	names []string
	err   error
}

type packageAddedMsg struct {
	name string
	res  plan.StepResult
	err  error
}

type packageIgnoredMsg struct {
	name string
	res  plan.StepResult
}

const (
	formRowSource = iota
	formRowMinimal
	formRowFull
	formRowGroup
	formRows
)

var sourceChoices = []manifest.Source{"", manifest.SourcePacman, manifest.SourceAUR}

type addForm struct {
	name    string
	source  int
	minimal bool
	full    bool
	group   textinput.Model
	row     int
	err     string
}

// Packages lists explicit packages that are neither in the catalog nor ignored, and adds or ignores them.
type Packages struct {
	manager PackageManager
	keys    KeyMap
	palette Palette

	names  []string
	loaded bool
	err    error
	notice string
	cursor int
	form   *addForm
}

func newPackages(deps Deps, keys KeyMap, palette Palette) *Packages {
	return &Packages{manager: deps.Packages, keys: keys, palette: palette}
}

func (p *Packages) Init() tea.Cmd  { return p.load() }
func (p *Packages) enter() tea.Cmd { return p.load() }

// Modal reports whether q and esc must close the add form instead of reaching the shell.
func (p *Packages) Modal() bool { return p.form != nil }

// capturesText reports whether printable keys belong to the group field.
func (p *Packages) capturesText() bool { return p.form != nil && p.form.row == formRowGroup }

func (p *Packages) load() tea.Cmd {
	if p.manager == nil {
		return nil
	}
	manager := p.manager
	return func() tea.Msg {
		names, err := manager.ListUnregistered(context.Background())
		return unregisteredMsg{names: names, err: err}
	}
}

func (p *Packages) selected() (string, bool) {
	if p.cursor >= len(p.names) {
		return "", false
	}
	return p.names[p.cursor], true
}

func (p *Packages) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case unregisteredMsg:
		p.loaded, p.names, p.err = true, msg.names, msg.err
		p.cursor = max(min(p.cursor, len(p.names)-1), 0)
	case packageAddedMsg:
		p.added(msg)
	case packageIgnoredMsg:
		p.ignored(msg)
	case tea.KeyPressMsg:
		if p.form != nil {
			return p, p.formKey(msg)
		}
		return p, p.listKey(msg)
	}
	return p, nil
}

func (p *Packages) listKey(msg tea.KeyPressMsg) tea.Cmd {
	name, ok := p.selected()
	switch {
	case key.Matches(msg, p.keys.Refresh):
		return p.load()
	case key.Matches(msg, p.keys.Up):
		p.cursor = max(p.cursor-1, 0)
	case key.Matches(msg, p.keys.Down):
		p.cursor = max(min(p.cursor+1, len(p.names)-1), 0)
	case !ok || p.manager == nil:
	case key.Matches(msg, p.keys.Add):
		p.notice = ""
		group := textinput.New()
		_ = group.Focus()
		p.form = &addForm{name: name, full: true, group: group}
	case key.Matches(msg, p.keys.Ignore):
		return p.ignore(name)
	}
	return nil
}

func (p *Packages) ignore(name string) tea.Cmd {
	manager := p.manager
	return func() tea.Msg {
		return packageIgnoredMsg{name: name, res: manager.IgnorePackage(context.Background(), name)}
	}
}

func (p *Packages) ignored(msg packageIgnoredMsg) {
	p.err = nil
	switch msg.res.Status {
	case plan.StepFailed:
		p.err = fmt.Errorf("ignore %s: %s", msg.name, failureText(msg.res))
	case plan.StepWouldApply:
		p.notice = msg.res.Detail
	default:
		p.drop(msg.name)
		p.notice = "Ignored " + msg.name
	}
}

func (p *Packages) added(msg packageAddedMsg) {
	if p.form == nil {
		return
	}
	switch {
	case msg.err != nil:
		p.form.err = msg.err.Error()
	case msg.res.Status == plan.StepFailed:
		p.form.err = failureText(msg.res)
	case msg.res.Status == plan.StepWouldApply:
		p.form, p.notice = nil, msg.res.Detail
	default:
		p.form, p.notice = nil, "Added "+msg.name+". It joins the Install selection after a restart."
		p.drop(msg.name)
	}
}

func (p *Packages) drop(name string) {
	p.names = slices.DeleteFunc(p.names, func(n string) bool { return n == name })
	p.cursor = max(min(p.cursor, len(p.names)-1), 0)
}

func failureText(res plan.StepResult) string {
	if res.Err != nil {
		return res.Err.Error()
	}
	return cmp.Or(res.Detail, "step failed")
}

func (p *Packages) formKey(msg tea.KeyPressMsg) tea.Cmd {
	f := p.form
	typing := f.row == formRowGroup
	switch {
	case key.Matches(msg, p.keys.Back), !typing && msg.String() == "q":
		p.form = nil
	case key.Matches(msg, p.keys.Confirm):
		return p.submit()
	case msg.String() == "up", !typing && key.Matches(msg, p.keys.Up):
		f.row = max(f.row-1, 0)
	case msg.String() == "down", !typing && key.Matches(msg, p.keys.Down):
		f.row = min(f.row+1, formRows-1)
	case typing:
		f.group, _ = f.group.Update(msg)
	case key.Matches(msg, p.keys.Toggle):
		f.toggle()
	}
	return nil
}

func (f *addForm) toggle() {
	switch f.row {
	case formRowSource:
		f.source = (f.source + 1) % len(sourceChoices)
	case formRowMinimal:
		f.minimal = !f.minimal
	case formRowFull:
		f.full = !f.full
	}
}

// submit rejects an empty source before anything is written.
func (p *Packages) submit() tea.Cmd {
	f := p.form
	if sourceChoices[f.source] == "" {
		f.err = "source is required: choose pacman or aur"
		return nil
	}
	f.err = ""
	pkg := manifest.Package{Name: f.name, Source: sourceChoices[f.source], Group: strings.TrimSpace(f.group.Value())}
	if f.minimal {
		pkg.Profiles = append(pkg.Profiles, manifest.ProfileMinimal)
	}
	if f.full {
		pkg.Profiles = append(pkg.Profiles, manifest.ProfileFull)
	}
	manager := p.manager
	return func() tea.Msg {
		res, err := manager.AddPackage(context.Background(), pkg)
		return packageAddedMsg{name: pkg.Name, res: res, err: err}
	}
}

func (p *Packages) View(width, height int) string {
	return renderPanel(width, height, p.body(width, max(height-2*panelPadY, 1)))
}

func (p *Packages) body(width, height int) string {
	if p.form != nil {
		return p.formView(width)
	}
	var lines []string
	if p.err != nil {
		lines = append(lines, paint(p.palette.Error, wrapToPanel(width, p.err.Error())))
	}
	if p.notice != "" {
		lines = append(lines, paint(p.palette.Warning, wrapToPanel(width, p.notice)))
	}
	muted := func(text string) string { return paint(p.palette.TextMuted, text) }
	switch {
	case p.manager == nil:
		return strings.Join(append(lines, muted("Packages are not available.")), "\n")
	case !p.loaded:
		return strings.Join(append(lines, muted("Loading...")), "\n")
	case p.err != nil && p.names == nil:
		return strings.Join(append(lines, muted("r refreshes.")), "\n")
	case len(p.names) == 0:
		return strings.Join(append(lines, muted("Every explicit package is registered or ignored.")), "\n")
	}
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d unregistered packages", len(p.names))))
	visible := max(height-len(lines)-2, 1)
	start := max(min(p.cursor-visible/2, len(p.names)-visible), 0)
	for i := start; i < min(start+visible, len(p.names)); i++ {
		lines = append(lines, cursorMark(i == p.cursor)+p.names[i])
	}
	return strings.Join(append(lines, "", muted("a adds to the catalog, i ignores, r refreshes")), "\n")
}

func (p *Packages) formView(width int) string {
	f := p.form
	source := string(sourceChoices[f.source])
	if source == "" {
		source = "(not chosen, space cycles pacman and aur)"
	}
	lines := []string{
		lipgloss.NewStyle().Bold(true).Render("Add " + f.name + " to the catalog"),
		cursorMark(f.row == formRowSource) + "source: " + source,
		cursorMark(f.row == formRowMinimal) + checkbox(f.minimal) + " minimal profile",
		cursorMark(f.row == formRowFull) + checkbox(f.full) + " full profile",
		cursorMark(f.row == formRowGroup) + "group (optional): " + f.group.View(),
	}
	if f.err != "" {
		lines = append(lines, "", paint(p.palette.Error, wrapToPanel(width, f.err)))
	}
	return strings.Join(append(lines, "", paint(p.palette.TextMuted, "up/down move, space toggles, enter adds, esc cancels")), "\n")
}
