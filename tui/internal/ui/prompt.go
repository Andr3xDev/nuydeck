package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

const choicePopupWidth = 30

// answer is what a key says to a yes/no question.
type answer int

const (
	answerNone answer = iota
	answerYes
	answerNo
)

// answer reads y as yes and n or esc as no. Any other key is no when otherDeclines, otherwise it is ignored.
func (k keyMap) answer(msg tea.KeyPressMsg, otherDeclines bool) answer {
	switch {
	case key.Matches(msg, k.Yes):
		return answerYes
	case key.Matches(msg, k.No, k.Back), otherDeclines:
		return answerNo
	}
	return answerNone
}

// choicePick is a popup that picks one of a few options; the cursor starts on options[cursor].
type choicePick struct {
	title   string
	options []string
	cursor  int
}

// update moves the cursor without wrapping; enter picks the cursor row and esc closes the popup.
func (c *choicePick) update(keys keyMap, msg tea.KeyPressMsg) (picked, closed bool) {
	switch {
	case key.Matches(msg, keys.Back):
		return false, true
	case key.Matches(msg, keys.Confirm):
		return true, false
	}
	c.cursor = max(min(c.cursor+keys.step(msg), len(c.options)-1), 0)
	return false, false
}

func (c *choicePick) popup(th theme, w int) popup {
	boxW := min(choicePopupWidth, w)
	rows := make([]listRow, len(c.options))
	for i, option := range c.options {
		rows[i] = listRow{text: option}
	}
	lines, _ := listLines(th, rows, c.cursor, contentWidth(boxW))
	h := len(lines) + borderCells
	return popup{box: renderBox(th, c.title, lines, boxW, h, true), w: boxW, h: h}
}
