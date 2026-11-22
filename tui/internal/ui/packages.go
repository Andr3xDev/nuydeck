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
	"github.com/Andr3xDev/nuydeck/tui/internal/packages"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
)

type packagesMsg struct {
	rows []packages.Row
	err  error
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

// packageFilters is the order f cycles through; the empty status shows every row.
var packageFilters = []packages.Status{"", packages.StatusMissing, packages.StatusDrift}

// Packages lists the catalog packages with their status on this machine plus the drift, and installs, uninstalls, adds or ignores them.
type Packages struct {
	svc  *shell
	deps PackagesDeps
	keys kit.KeyMap
	th   kit.Theme

	rows   []packages.Row
	marks  map[string]bool // keyed by package name
	filter int             // index into packageFilters
	loaded bool
	err    error
	notice string
	cursor int
	form   *addForm
	flow   *flow

	ownRunPending bool // a run started here is open, so its close needs a reload
}

func newPackages(svc *shell) *Packages {
	return &Packages{svc: svc, deps: svc.Packages, keys: svc.keys, th: svc.th, marks: map[string]bool{}}
}

func (p *Packages) Title() string { return "Packages" }

func (p *Packages) Init() tea.Cmd  { return p.load() }
func (p *Packages) Enter() tea.Cmd { return p.load() }

// Keys claims esc while the add form or a flow is open, so esc steps back instead of leaving the panel.
func (p *Packages) Keys() []key.Binding {
	if p.flow.active() {
		return p.flow.Keys()
	}
	if p.form != nil {
		return []key.Binding{p.keys.Up, p.keys.Down, p.keys.Toggle, p.keys.Confirm, p.keys.Back}
	}
	keys := append(p.keys.Move(), p.keys.Toggle, p.keys.All, p.keys.Filter, p.keys.Refresh)
	if p.canRun() {
		keys = append(keys, p.keys.Install, p.keys.Uninstall)
	}
	if p.wired() {
		keys = append(keys, p.keys.Add, p.keys.Ignore)
	}
	return keys
}

// CapturesText reports whether printable keys belong to the group field.
func (p *Packages) CapturesText() bool { return p.form != nil && p.form.row == formRowGroup }

// wired is false when the package manager is not available.
func (p *Packages) wired() bool {
	return p.deps.List != nil && p.deps.AddPackage != nil && p.deps.IgnorePackage != nil
}

func (p *Packages) canRun() bool { return p.deps.List != nil && p.svc.RunPlan != nil }

func (p *Packages) load() tea.Cmd {
	if p.deps.List == nil {
		return nil
	}
	list := p.deps.List
	return func() tea.Msg {
		rows, err := list(context.Background())
		return packagesMsg{rows: rows, err: err}
	}
}

// visible is the rows the filter lets through.
func (p *Packages) visible() []packages.Row {
	status := packageFilters[p.filter]
	if status == "" {
		return p.rows
	}
	return slices.DeleteFunc(slices.Clone(p.rows), func(r packages.Row) bool { return r.Status != status })
}

func (p *Packages) selected() (packages.Row, bool) {
	rows := p.visible()
	if p.cursor >= len(rows) {
		return packages.Row{}, false
	}
	return rows[p.cursor], true
}

// targets are the marked visible rows, or the cursor row when none is marked.
func (p *Packages) targets() []packages.Row {
	marked := slices.DeleteFunc(slices.Clone(p.visible()), func(r packages.Row) bool { return !p.marks[r.Name] })
	if len(marked) > 0 {
		return marked
	}
	if sel, ok := p.selected(); ok {
		return []packages.Row{sel}
	}
	return nil
}

func (p *Packages) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case packagesMsg:
		p.receive(msg)
	case packageAddedMsg:
		return p, p.added(msg)
	case packageIgnoredMsg:
		return p, p.ignored(msg)
	case runClosed:
		if p.ownRunPending {
			p.ownRunPending = false
			clear(p.marks)
			return p, p.load()
		}
	case plannedMsg:
		if p.flow.active() {
			return p, p.flow.Update(msg)
		}
	case tea.KeyPressMsg:
		switch {
		case p.flow.active():
			return p, p.flow.Update(msg)
		case p.form != nil:
			return p, p.formKey(msg)
		}
		return p, p.listKey(msg)
	}
	return p, nil
}

func (p *Packages) receive(msg packagesMsg) {
	p.loaded, p.rows, p.err = true, msg.rows, msg.err
	for name := range p.marks {
		if !slices.ContainsFunc(p.rows, func(r packages.Row) bool { return r.Name == name }) {
			delete(p.marks, name)
		}
	}
	p.cursor = max(min(p.cursor, len(p.visible())-1), 0)
}

func (p *Packages) listKey(msg tea.KeyPressMsg) tea.Cmd {
	if delta := p.keys.Step(msg); delta != 0 {
		p.cursor = kit.MoveCursor(p.cursor, delta, len(p.visible()))
		return nil
	}
	sel, ok := p.selected()
	switch {
	case key.Matches(msg, p.keys.Refresh):
		return p.load()
	case key.Matches(msg, p.keys.Filter):
		p.filter = (p.filter + 1) % len(packageFilters)
		p.cursor = 0
		clear(p.marks)
	case !ok:
	case key.Matches(msg, p.keys.Toggle):
		p.marks[sel.Name] = !p.marks[sel.Name]
	case key.Matches(msg, p.keys.All):
		p.markAll()
	case key.Matches(msg, p.keys.Install) && p.canRun():
		return p.startPlan(packages.PlanInstall)
	case key.Matches(msg, p.keys.Uninstall) && p.canRun():
		return p.startPlan(packages.PlanUninstall)
	case key.Matches(msg, p.keys.Add) && p.wired():
		if !p.checkDrift(sel) {
			return nil
		}
		p.notice = ""
		group := textinput.New()
		_ = group.Focus()
		p.form = &addForm{name: sel.Name, full: true, group: group}
	case key.Matches(msg, p.keys.Ignore) && p.wired():
		if !p.checkDrift(sel) {
			return nil
		}
		return p.ignore(sel.Name)
	}
	return nil
}

// markAll marks every visible row, or clears them when all are already marked.
func (p *Packages) markAll() {
	rows := p.visible()
	all := !slices.ContainsFunc(rows, func(r packages.Row) bool { return !p.marks[r.Name] })
	for _, r := range rows {
		p.marks[r.Name] = !all
	}
}

// checkDrift sets a notice and returns false for a row that is not drift: only drift rows can join the manifest or be ignored.
func (p *Packages) checkDrift(row packages.Row) bool {
	if row.Status == packages.StatusDrift {
		return true
	}
	p.notice = fmt.Sprintf("%s is not drift: m and g only apply to drift rows", row.Name)
	return false
}

// startPlan previews and runs a plan for the targets; the domain's notices go to the notice line.
func (p *Packages) startPlan(planFor func([]packages.Row) (plan.Plan, []string)) tea.Cmd {
	built, notices := planFor(p.targets())
	p.notice, p.err = strings.Join(notices, "; "), nil
	if built.Empty() {
		return nil
	}
	p.flow = newFlow(p.svc, flowSource{
		Title: built.Title,
		Plan:  func(context.Context, []string) (staged, error) { return newPlanStaged(built), nil },
		Run: func(confirmed plan.Plan) runRequest {
			p.ownRunPending = true
			return planRunRequest(p.svc, confirmed)
		},
	})
	return p.flow.start()
}

func (p *Packages) ignore(name string) tea.Cmd {
	ignorePackage := p.deps.IgnorePackage
	return func() tea.Msg {
		return packageIgnoredMsg{name: name, res: ignorePackage(context.Background(), name)}
	}
}

func (p *Packages) ignored(msg packageIgnoredMsg) tea.Cmd {
	p.err = nil
	switch msg.res.Status {
	case plan.StepFailed:
		p.err = fmt.Errorf("ignore %s: %s", msg.name, failureText(msg.res))
	case plan.StepWouldApply:
		p.notice = msg.res.Detail
	default:
		p.notice = "Ignored " + msg.name
		return p.load()
	}
	return nil
}

func (p *Packages) added(msg packageAddedMsg) tea.Cmd {
	if p.form == nil {
		return nil
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
		return p.load()
	}
	return nil
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
	if p.flow.active() {
		return p.flow.box(p.Title(), width, height)
	}
	cw := kit.ContentWidth(width)
	if p.form != nil {
		return screenBox(p.th, "Packages", p.formView(cw), width, height)
	}
	var lines []string
	if p.err != nil {
		lines = append(lines, styled(p.th.Err, kit.Wrap(p.err.Error(), cw))...)
	}
	if p.notice != "" {
		lines = append(lines, styled(p.th.Warning, kit.Wrap(p.notice, cw))...)
	}
	muted := func(text string) []string { return styled(p.th.Muted, kit.Wrap(text, cw)) }
	var msg string
	switch {
	case !p.wired():
		msg = "Packages are not available."
	case !p.loaded:
		msg = "Loading..."
	case p.err != nil && p.rows == nil:
		msg = "r refreshes."
	case len(p.rows) == 0:
		msg = "The catalog is empty and every explicit package is ignored."
	}
	if msg != "" {
		return screenBox(p.th, "Packages", append(lines, muted(msg)...), width, height)
	}
	lines = append(lines, p.th.Title.Render(p.header()))
	visible := p.visible()
	if len(visible) == 0 {
		return screenBox(p.th, "Packages", append(lines, muted("No packages match the filter, f changes it.")...), width, height)
	}
	rows := make([]kit.TableRow, len(visible))
	for i, r := range visible {
		rows[i] = kit.TableRow{Cells: []string{checkbox(p.marks[r.Name]) + " " + r.Name, r.Group, string(r.Status)}}
	}
	header, list, at := kit.TableLines(p.th, packagesTableColumns, rows, p.cursor, cw)
	lines = append(lines, header)
	lines = append(lines, kit.WindowAround(list, at, max(innerHeight(height)-len(lines), 1))...)
	return screenBox(p.th, "Packages", lines, width, height)
}

var packagesTableColumns = []kit.Column{
	{Title: "Package"},
	{Title: "Group", Priority: 2},
	{Title: "Status", Priority: 1},
}

// header counts every row by status and names the active filter.
func (p *Packages) header() string {
	counts := map[packages.Status]int{}
	for _, r := range p.rows {
		counts[r.Status]++
	}
	text := fmt.Sprintf("%d packages: %d installed, %d missing, %d drift", len(p.rows),
		counts[packages.StatusInstalled], counts[packages.StatusMissing], counts[packages.StatusDrift])
	if status := packageFilters[p.filter]; status != "" {
		text += ", showing " + string(status)
	}
	return text
}

func (p *Packages) formView(cw int) []string {
	f := p.form
	source := string(sourceChoices[f.source])
	if source == "" {
		source = "(not chosen, space cycles pacman and aur)"
	}
	lines := []string{
		p.th.Title.Render(kit.Truncate(kit.SanitizeLine("Add "+f.name+" to the catalog"), cw)),
		cursorMark(f.row == formRowSource) + "source: " + source,
		cursorMark(f.row == formRowMinimal) + checkbox(f.minimal) + " minimal profile",
		cursorMark(f.row == formRowFull) + checkbox(f.full) + " full profile",
		cursorMark(f.row == formRowGroup) + "group (optional): " + f.group.View(),
	}
	lines = clip(lines, cw)
	if f.err != "" {
		lines = append(lines, "")
		lines = append(lines, styled(p.th.Err, kit.Wrap(f.err, cw))...)
	}
	lines = append(lines, "")
	return append(lines, styled(p.th.Muted, kit.Wrap("up/down move, space toggles, enter adds, esc cancels", cw))...)
}
