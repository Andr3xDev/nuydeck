package kit

import tea "charm.land/bubbletea/v2"

var (
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyUp    = tea.KeyPressMsg{Code: tea.KeyUp}
)

func keyPress(code rune) tea.KeyPressMsg {
	if code == tea.KeyEnter {
		return tea.KeyPressMsg{Code: code}
	}
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}
