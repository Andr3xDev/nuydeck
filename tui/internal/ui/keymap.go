package ui

import "charm.land/bubbles/v2/key"

// KeyMap lists every key the TUI reacts to; it also feeds the help overlay.
type KeyMap struct {
	Quit    key.Binding
	Help    key.Binding
	Back    key.Binding
	Refresh key.Binding
	Up      key.Binding
	Down    key.Binding
	Focus   key.Binding
	Toggle  key.Binding
	All     key.Binding
	Confirm key.Binding
	Pull    key.Binding
	Push    key.Binding
}

// DefaultKeyMap returns the keybindings documented in the tui-shell spec.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Focus:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
		Toggle:  key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
		All:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle all")),
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Pull:    key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pull")),
		Push:    key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "push")),
	}
}

// ShortHelp implements help.KeyMap for the keybar.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Focus, k.Refresh, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap for the help overlay.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Focus, k.Confirm, k.Back},
		{k.Toggle, k.All, k.Pull, k.Push},
		{k.Refresh, k.Help, k.Quit},
	}
}
