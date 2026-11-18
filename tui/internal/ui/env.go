package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/envvars"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	envPopupTitle     = "Env source"
	chooseSourceText  = "Choose a source with s: the repo copy or the live copy."
	noVariablesText   = "No variables: the file has no hl.env lines."
	notListableText   = "Cannot list variables: the file is not a readable text file."
	envFileMissingFmt = "File missing: %s"
)

// envSourceOptions follow the order of envvars.SourceRepo and envvars.SourceLive.
var envSourceOptions = []string{"repo copy", "live copy"}

type envEditedMsg struct {
	preview string
	err     error
}

// Env lists the hl.env variables of one copy of the Hyprland env file and hands that copy to the editor.
type Env struct {
	svc    *shell
	loc    envvars.Locations
	locErr error
	pick   *choicePick // the open source popup, nil when closed
	source envvars.Source
	chosen bool
	vars   []envvars.Var
	cursor int
	msg    string // why the list is empty
	// preview is the dry-run editor line.
	preview string
	err     error
}

func newEnv(svc *shell) *Env {
	loc, err := envvars.Paths(svc.Entries)
	return &Env{svc: svc, loc: loc, locErr: err}
}

func (e *Env) Title() string { return "Env" }

// Enter asks for the source again every time: the choice is not kept.
func (e *Env) Enter() tea.Cmd {
	e.chosen, e.vars, e.cursor, e.msg, e.preview, e.err = false, nil, 0, chooseSourceText, "", nil
	e.openPopup()
	return nil
}

func (e *Env) openPopup() {
	if e.locErr != nil {
		return
	}
	e.pick = &choicePick{title: envPopupTitle, options: envSourceOptions, cursor: int(e.source)}
}

// Keys claims esc while the popup is open, so esc closes it instead of leaving the panel.
func (e *Env) Keys() []key.Binding {
	k := e.svc.keys
	switch {
	case e.pick != nil:
		return []key.Binding{k.Up, k.Down, k.Confirm, k.Back}
	case !e.chosen:
		return []key.Binding{described(k.Start, "source")}
	}
	return []key.Binding{k.Up, k.Down, described(k.Start, "source"), k.Edit, k.Refresh}
}

// reload reads the chosen copy; nothing is read before a source is chosen.
func (e *Env) reload() {
	e.vars, e.msg, e.err = nil, "", nil
	path := e.loc.Path(e.source)
	side, err := e.svc.ReadSide(path)
	switch {
	case err != nil:
		e.err = err
	case !side.Exists:
		e.msg = fmt.Sprintf(envFileMissingFmt, path)
	case side.Kind != plan.KindFile || side.Binary || side.TooLarge:
		e.msg = notListableText
	default:
		if e.vars = envvars.ParseHlEnv(string(side.Content)); len(e.vars) == 0 {
			e.msg = noVariablesText
		}
	}
	e.cursor = max(min(e.cursor, len(e.vars)-1), 0)
}

func (e *Env) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case envEditedMsg:
		e.preview, e.err = msg.preview, msg.err
		if msg.err == nil {
			e.reload()
		}
	case tea.KeyPressMsg:
		return e, e.handleKey(msg)
	}
	return e, nil
}

func (e *Env) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := e.svc.keys
	if e.pick != nil {
		picked, closed := e.pick.update(k, msg)
		if picked {
			e.source, e.chosen, e.cursor = envvars.Source(e.pick.cursor), true, 0
			e.reload()
		}
		if picked || closed {
			e.pick = nil
		}
		return nil
	}
	switch {
	case key.Matches(msg, k.Start):
		e.openPopup()
	case !e.chosen:
	case key.Matches(msg, k.Refresh):
		e.reload()
	case key.Matches(msg, k.Up):
		e.cursor = max(e.cursor-1, 0)
	case key.Matches(msg, k.Down):
		e.cursor = max(min(e.cursor+1, len(e.vars)-1), 0)
	case key.Matches(msg, k.Edit):
		return e.edit()
	}
	return nil
}

func (e *Env) edit() tea.Cmd {
	edit, ctx, path := e.svc.Env.Edit, e.svc.ctx, e.loc.Path(e.source)
	e.preview, e.err = "", nil
	return func() tea.Msg {
		preview, err := edit(ctx, path)
		return envEditedMsg{preview: preview, err: err}
	}
}

func (e *Env) View(width, height int) string {
	th, cw := e.svc.th, contentWidth(width)
	var lines []string
	switch {
	case e.locErr != nil:
		lines = styled(th.err, wrap(e.locErr.Error(), cw))
	case e.chosen:
		lines = append(lines, th.title.Render(truncate(sanitizeLine(fmt.Sprintf("%s: %s", envSourceOptions[e.source], e.loc.Path(e.source))), cw)))
	}
	if e.err != nil {
		lines = append(lines, styled(th.err, wrap(e.err.Error(), cw))...)
	}
	if e.preview != "" {
		lines = append(lines, styled(th.warning, wrap(e.preview, cw))...)
	}
	if e.msg != "" && e.locErr == nil {
		lines = append(lines, styled(th.muted, wrap(e.msg, cw))...)
	}
	if len(e.vars) > 0 {
		rows := make([]listRow, len(e.vars))
		for i, v := range e.vars {
			rows[i] = listRow{text: v.Name + "  " + v.Value}
		}
		list, at := listLines(th, rows, e.cursor, cw)
		lines = append(lines, windowAround(list, at, max(innerHeight(height)-len(lines), 1))...)
	}
	base := screenBox(th, e.Title(), lines, width, height)
	if e.pick == nil {
		return base
	}
	return overlayCentered(base, e.pick.popup(th, width), width, height)
}
