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

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

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
	deps PackagesDeps
	keys keyMap
	th   theme

	names  []string
	loaded bool
	err    error
	notice string
	cursor int
	form   *addForm
}

func newPackages(svc *services) *Packages {
	return &Packages{deps: svc.Packages, keys: svc.keys, th: svc.th}
}

func (p *Packages) Title() string { return "Packages" }

func (p *Packages) Init() tea.Cmd  { return p.load() }
func (p *Packages) Enter() tea.Cmd { return p.load() }

// Keys claims esc while the add form is open, so esc closes it instead of leaving the panel.
func (p *Packages) Keys() []key.Binding {
	if p.form != nil {
		return []key.Binding{p.keys.Up, p.keys.Down, p.keys.Toggle, p.keys.Confirm, p.keys.Back}
	}
	return []key.Binding{p.keys.Up, p.keys.Down, p.keys.Refresh, p.keys.Add, p.keys.Ignore}
}

// CapturesText reports whether printable keys belong to the group field.
func (p *Packages) CapturesText() bool { return p.form != nil && p.form.row == formRowGroup }

// wired is false when the package manager is not available.
func (p *Packages) wired() bool {
	return p.deps.ListUnregistered != nil && p.deps.AddPackage != nil && p.deps.IgnorePackage != nil
}

func (p *Packages) load() tea.Cmd {
	if p.deps.ListUnregistered == nil {
		return nil
	}
	listUnregistered := p.deps.ListUnregistered
	return func() tea.Msg {
		names, err := listUnregistered(context.Background())
		return unregisteredMsg{names: names, err: err}
	}
}

func (p *Packages) selected() (string, bool) {
	if p.cursor >= len(p.names) {
		return "", false
	}
	return p.names[p.cursor], true
}

func (p *Packages) Update(msg tea.Msg) (screen, tea.Cmd) {
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
	case !ok || !p.wired():
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
	ignorePackage := p.deps.IgnorePackage
	return func() tea.Msg {
		return packageIgnoredMsg{name: name, res: ignorePackage(context.Background(), name)}
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
	case key.Matches(msg, p.keys.Back):
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
	pkg := manifest.Package{Name: f.name, Source: sourceChoices[f.source], Group: strings.TrimSpace(f.group.Value())}
	if !pkg.ValidSource() {
		f.err = "source is required: choose pacman or aur"
		return nil
	}
	f.err = ""
	if f.minimal {
		pkg.Profiles = append(pkg.Profiles, manifest.ProfileMinimal)
	}
	if f.full {
		pkg.Profiles = append(pkg.Profiles, manifest.ProfileFull)
	}
	addPackage := p.deps.AddPackage
	return func() tea.Msg {
		res, err := addPackage(context.Background(), pkg)
		return packageAddedMsg{name: pkg.Name, res: res, err: err}
	}
}

func (p *Packages) View(width, height int) string {
	cw := contentWidth(width)
	if p.form != nil {
		return screenBox(p.th, "Packages", p.formView(cw), width, height)
	}
	var lines []string
	if p.err != nil {
		lines = append(lines, styled(p.th.err, wrap(p.err.Error(), cw))...)
	}
	if p.notice != "" {
		lines = append(lines, styled(p.th.warning, wrap(p.notice, cw))...)
	}
	muted := func(text string) []string { return styled(p.th.muted, wrap(text, cw)) }
	var msg string
	switch {
	case !p.wired():
		msg = "Packages are not available."
	case !p.loaded:
		msg = "Loading..."
	case p.err != nil && p.names == nil:
		msg = "r refreshes."
	case len(p.names) == 0:
		msg = "Every explicit package is registered or ignored."
	}
	if msg != "" {
		return screenBox(p.th, "Packages", append(lines, muted(msg)...), width, height)
	}
	lines = append(lines, p.th.title.Render(fmt.Sprintf("%d unregistered packages", len(p.names))))
	rows := make([]listRow, len(p.names))
	for i, name := range p.names {
		rows[i] = listRow{text: name}
	}
	list, at := listLines(p.th, rows, p.cursor, cw)
	lines = append(lines, windowAround(list, at, max(innerHeight(height)-len(lines)-2, 1))...)
	lines = append(lines, "")
	lines = append(lines, muted("a adds to the catalog, i ignores, r refreshes")...)
	return screenBox(p.th, "Packages", lines, width, height)
}

func (p *Packages) formView(cw int) []string {
	f := p.form
	source := string(sourceChoices[f.source])
	if source == "" {
		source = "(not chosen, space cycles pacman and aur)"
	}
	lines := []string{
		p.th.title.Render(truncate(sanitizeLine("Add"+f.name+" to the catalog"), cw)),
		cursorMark(f.row == formRowSource) + "source: " + source,
		cursorMark(f.row == formRowMinimal) + checkbox(f.minimal) + " minimal profile",
		cursorMark(f.row == formRowFull) + checkbox(f.full) + " full profile",
		cursorMark(f.row == formRowGroup) + "group (optional): " + f.group.View(),
	}
	lines = clip(lines, cw)
	if f.err != "" {
		lines = append(lines, "")
		lines = append(lines, styled(p.th.err, wrap(f.err, cw))...)
	}
	lines = append(lines, "")
	return append(lines, styled(p.th.muted, wrap("up/down move, space toggles, enter adds, esc cancels", cw))...)
}
