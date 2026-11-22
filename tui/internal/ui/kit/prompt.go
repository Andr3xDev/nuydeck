package kit

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

const choicePopupWidth = 30

// Answer is what a key says to a yes/no question.
type Answer int

const (
	AnswerNone Answer = iota
	AnswerYes
	AnswerNo
)

// Answer reads y as yes and n or esc as no. Any other key is no when otherDeclines, otherwise it is ignored.
func (k KeyMap) Answer(msg tea.KeyPressMsg, otherDeclines bool) Answer {
	switch {
	case key.Matches(msg, k.Yes):
		return AnswerYes
	case key.Matches(msg, k.No, k.Back), otherDeclines:
		return AnswerNo
	}
	return AnswerNone
}

// ChoicePick is a popup that picks one of a few options; the cursor starts on Options[Cursor].
type ChoicePick struct {
	Title   string
	Options []string
	Cursor  int
}

// Update moves the cursor without wrapping; enter picks the cursor row and esc closes the popup.
func (c *ChoicePick) Update(keys KeyMap, msg tea.KeyPressMsg) (picked, closed bool) {
	switch {
	case key.Matches(msg, keys.Back):
		return false, true
	case key.Matches(msg, keys.Confirm):
		return true, false
	}
	c.Cursor = MoveCursor(c.Cursor, keys.Step(msg), len(c.Options))
	return false, false
}

// Popup renders the options as a list popup no wider than w.
func (c *ChoicePick) Popup(th Theme, w int) Popup {
	boxW := min(choicePopupWidth, w)
	rows := make([]ListRow, len(c.Options))
	for i, option := range c.Options {
		rows[i] = ListRow{Text: option}
	}
	lines, _ := ListLines(th, rows, c.Cursor, ContentWidth(boxW))
	h := len(lines) + BorderCells
	return Popup{box: RenderBox(th, c.Title, lines, boxW, h, true), w: boxW, h: h}
}
