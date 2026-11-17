package ui

import (
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// keyMap holds the bindings shared by the shell and the screens; each screen's Keys() picks the ones its stage answers to.
type keyMap struct {
	Up, Down                 key.Binding
	Focus, Open, Back        key.Binding
	Refresh, Help, Quit      key.Binding
	Toggle, All, Confirm     key.Binding
	Yes, No                  key.Binding
	Pull, Push, Delete       key.Binding
	Add, Ignore, SkipSecrets key.Binding
	CancelRun                key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
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
		Add:         key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add package")),
		Ignore:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "ignore package")),
		SkipSecrets: key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "skip secrets file")),
		CancelRun:   key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "cancel")),
	}
}

// step is the cursor delta a movement key asks for, or 0 for any other key.
func (k keyMap) step(msg tea.KeyPressMsg) int {
	switch {
	case key.Matches(msg, k.Up):
		return -1
	case key.Matches(msg, k.Down):
		return 1
	}
	return 0
}

// described returns b with another help text, for keys whose meaning depends on the stage.
func described(b key.Binding, desc string) key.Binding {
	b.SetHelp(b.Help().Key, desc)
	return b
}

// claims reports whether one of the bindings answers to the key.
func claims(bindings []key.Binding, k string) bool {
	return slices.ContainsFunc(bindings, func(b key.Binding) bool { return slices.Contains(b.Keys(), k) })
}

// pinnedKeys stay visible at the right of the keybar however many keys the focus has.
func (k keyMap) pinnedKeys() []key.Binding { return []key.Binding{k.Help, k.Quit} }

// sidebarKeys are the keys valid at sidebar focus.
func (k keyMap) sidebarKeys() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Open, k.Focus}
}

// panelKeys puts the screen's keys first. r stays "refresh" unless the screen claims it;
// esc goes back to the sidebar unless the screen claims it (a dialog or a later wizard page).
func (k keyMap) panelKeys(screenKeys []key.Binding) []key.Binding {
	keys := slices.Clone(screenKeys)
	if !claims(screenKeys, "r") {
		keys = append(keys, k.Refresh)
	}
	if !claims(screenKeys, "esc") {
		keys = append(keys, k.Back)
	}
	return keys
}
