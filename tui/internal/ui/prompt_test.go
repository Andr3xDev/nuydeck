package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const popupTestWidth = 60

func TestKeyMapAnswerMapsYNAndEscAndTheOtherKeysByPolicy(t *testing.T) {
	keys := newKeyMap()
	cases := []struct {
		name                       string
		msg                        tea.KeyPressMsg
		wantIgnoring, wantDeclines answer
	}{
		{"y", keyPress('y'), answerYes, answerYes},
		{"n", keyPress('n'), answerNo, answerNo},
		{"esc", keyEsc, answerNo, answerNo},
		{"other key", keyPress('x'), answerNone, answerNo},
		{"enter", keyEnter, answerNone, answerNo},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := keys.answer(tt.msg, false); got != tt.wantIgnoring {
				t.Errorf("answer(otherDeclines=false) = %v, want %v", got, tt.wantIgnoring)
			}
			if got := keys.answer(tt.msg, true); got != tt.wantDeclines {
				t.Errorf("answer(otherDeclines=true) = %v, want %v", got, tt.wantDeclines)
			}
		})
	}
}

func TestChoicePickMovesWithoutWrappingPicksOnEnterAndClosesOnEsc(t *testing.T) {
	keys := newKeyMap()
	newPick := func() *choicePick {
		return &choicePick{title: "Mode", options: []string{"Normal", "Hard", "Other"}}
	}
	cases := []struct {
		name       string
		presses    []tea.KeyPressMsg
		wantCursor int
		picked     bool
		closed     bool
	}{
		{"up at the top stays", []tea.KeyPressMsg{keyPress('k')}, 0, false, false},
		{"down moves", []tea.KeyPressMsg{keyPress('j')}, 1, false, false},
		{"J jumps to the last row", []tea.KeyPressMsg{keyPress('J')}, 2, false, false},
		{"down at the bottom stays", []tea.KeyPressMsg{keyPress('j'), keyPress('j'), keyPress('j')}, 2, false, false},
		{"enter picks the cursor row", []tea.KeyPressMsg{keyPress('j'), keyEnter}, 1, true, false},
		{"esc closes", []tea.KeyPressMsg{keyPress('j'), keyEsc}, 1, false, true},
		{"other key does nothing", []tea.KeyPressMsg{keyPress('x')}, 0, false, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pick := newPick()
			var picked, closed bool
			for _, k := range tt.presses {
				picked, closed = pick.update(keys, k)
			}
			if pick.cursor != tt.wantCursor || picked != tt.picked || closed != tt.closed {
				t.Errorf("cursor=%d picked=%v closed=%v, want cursor=%d picked=%v closed=%v", pick.cursor, picked, closed, tt.wantCursor, tt.picked, tt.closed)
			}
		})
	}
}

func TestChoicePickPopupShowsTheTitleAndEveryOption(t *testing.T) {
	pick := &choicePick{title: "Mode", options: []string{"Normal", "Hard"}}

	box := ansi.Strip(pick.popup(ForVariant(""), popupTestWidth).box)

	for _, want := range []string{"Mode", "Normal", "Hard"} {
		if !strings.Contains(box, want) {
			t.Errorf("popup lacks %q:\n%s", want, box)
		}
	}
}
