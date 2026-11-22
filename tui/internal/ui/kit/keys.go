package kit

import (
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// KeyMap holds the bindings shared by the shell and the screens; each screen's Keys() picks the ones its stage answers to.
type KeyMap struct {
	Up, Down                 key.Binding
	Focus, Open, Back        key.Binding
	Refresh, Help, Quit      key.Binding
	Toggle, All, Confirm     key.Binding
	Yes, No                  key.Binding
	Pull, Push, Delete       key.Binding
	Add, Ignore, SkipSecrets key.Binding
	CancelRun                key.Binding
	Edit, Diff               key.Binding
	Start, Stop, Restart     key.Binding
	Mask, Unmask, Logs, Link key.Binding
	JumpUp, JumpDown         key.Binding
	Filter, Snapshot         key.Binding
	Restore, Install         key.Binding
	Uninstall                key.Binding
}

// jumpRows is how many rows K and J move the cursor at once.
const jumpRows = 5

// NewKeyMap returns the default key bindings.
func NewKeyMap() KeyMap {
	return KeyMap{
		Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Focus:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
		Open:        key.NewBinding(key.WithKeys("enter", "right", "l"), key.WithHelp("enter", "open panel")),
		Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Toggle:      key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
		All:         key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle all")),
		Confirm:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Yes:         key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
		No:          key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no")),
		Pull:        key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pull")),
		Push:        key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "push")),
		Delete:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete backup")),
		Add:         key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "add to manifest")),
		Ignore:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "ignore")),
		JumpUp:      key.NewBinding(key.WithKeys("K", "shift+k"), key.WithHelp("K", "jump up")),
		JumpDown:    key.NewBinding(key.WithKeys("J", "shift+j"), key.WithHelp("J", "jump down")),
		Filter:      key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
		Snapshot:    key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "manual backup")),
		Restore:     key.NewBinding(key.WithKeys("enter", "r"), key.WithHelp("enter", "restore")),
		Install:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "install")),
		Uninstall:   key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "uninstall")),
		SkipSecrets: key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "skip secrets file")),
		CancelRun:   key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "cancel")),
		Edit:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Diff:        key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view diff")),
		Start:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
		Stop:        key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
		Restart:     key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "restart")),
		Mask:        key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mask")),
		Unmask:      key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unmask")),
		Logs:        key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "logs")),
		Link:        key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open link")),
	}
}

// Arrow is the one-row cursor delta of an up/down key, or 0 for any other key (J and K included).
func (k KeyMap) Arrow(msg tea.KeyPressMsg) int {
	switch {
	case key.Matches(msg, k.Up):
		return -1
	case key.Matches(msg, k.Down):
		return 1
	}
	return 0
}

// Move lists the cursor-movement bindings for a screen's Keys().
func (k KeyMap) Move() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.JumpUp, k.JumpDown}
}

// Step is the cursor delta a movement key asks for, or 0 for any other key.
func (k KeyMap) Step(msg tea.KeyPressMsg) int {
	if delta := k.Arrow(msg); delta != 0 {
		return delta
	}
	switch {
	case key.Matches(msg, k.JumpUp):
		return -jumpRows
	case key.Matches(msg, k.JumpDown):
		return jumpRows
	}
	return 0
}

// MoveCursor applies a step to a cursor over n rows, staying inside the list.
func MoveCursor(cursor, delta, n int) int {
	return max(min(cursor+delta, n-1), 0)
}

// Described returns b with another help text, for keys whose meaning depends on the stage.
func Described(b key.Binding, desc string) key.Binding {
	b.SetHelp(b.Help().Key, desc)
	return b
}

// Claims reports whether one of the bindings answers to the key.
func Claims(bindings []key.Binding, k string) bool {
	return slices.ContainsFunc(bindings, func(b key.Binding) bool { return slices.Contains(b.Keys(), k) })
}

// PanelKeys puts the screen's keys first; esc goes back to the sidebar unless the screen claims it (a dialog or a later wizard page).
func (k KeyMap) PanelKeys(screenKeys []key.Binding) []key.Binding {
	keys := slices.Clone(screenKeys)
	if !Claims(screenKeys, "esc") {
		keys = append(keys, k.Back)
	}
	return keys
}
