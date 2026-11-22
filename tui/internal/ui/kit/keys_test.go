package kit

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestKeyMapStep(t *testing.T) {
	k := NewKeyMap()
	cases := []struct {
		name string
		msg  tea.KeyPressMsg
		want int
	}{
		{"up arrow", keyUp, -1},
		{"k", keyPress('k'), -1},
		{"down arrow", keyDown, 1},
		{"j", keyPress('j'), 1},
		{"K jumps up", keyPress('K'), -jumpRows},
		{"shift+k jumps up", tea.KeyPressMsg{Code: 'k', Mod: tea.ModShift}, -jumpRows},
		{"J jumps down", keyPress('J'), jumpRows},
		{"shift+j jumps down", tea.KeyPressMsg{Code: 'j', Mod: tea.ModShift}, jumpRows},
		{"other key", keyPress('x'), 0},
	}
	for _, c := range cases {
		if got := k.Step(c.msg); got != c.want {
			t.Errorf("%s: step = %d, want %d", c.name, got, c.want)
		}
		wantArrow := c.want
		if wantArrow != 1 && wantArrow != -1 {
			wantArrow = 0
		}
		if got := k.Arrow(c.msg); got != wantArrow {
			t.Errorf("%s: arrow = %d, want %d", c.name, got, wantArrow)
		}
	}
	if got, want := k.Move(), []key.Binding{k.Up, k.Down, k.JumpUp, k.JumpDown}; len(got) != len(want) {
		t.Errorf("move() = %d bindings, want %d", len(got), len(want))
	} else {
		for i := range want {
			if !slices.Equal(got[i].Keys(), want[i].Keys()) {
				t.Errorf("move()[%d] keys = %v, want %v", i, got[i].Keys(), want[i].Keys())
			}
		}
	}
	if jumpRows != 5 {
		t.Errorf("jumpRows = %d, want 5", jumpRows)
	}
}

func TestMoveCursor(t *testing.T) {
	cases := []struct {
		name          string
		cursor, delta int
		n, want       int
	}{
		{"one down", 2, 1, 10, 3},
		{"one up", 2, -1, 10, 1},
		{"jump down", 0, 5, 10, 5},
		{"jump up", 7, -5, 10, 2},
		{"clamped at the end", 8, 5, 10, 9},
		{"clamped at the start", 2, -5, 10, 0},
		{"already at the end", 9, 1, 10, 9},
		{"already at the start", 0, -1, 10, 0},
		{"empty list down", 0, 5, 0, 0},
		{"empty list up", 0, -5, 0, 0},
	}
	for _, c := range cases {
		if got := MoveCursor(c.cursor, c.delta, c.n); got != c.want {
			t.Errorf("%s: moveCursor(%d, %d, %d) = %d, want %d", c.name, c.cursor, c.delta, c.n, got, c.want)
		}
	}
}

func TestPanelKeysFollowTheClaimRules(t *testing.T) {
	k := NewKeyMap()
	restore := Described(key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restore")), "restore")
	other := key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "other"))
	closeFlow := key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))

	plain := k.PanelKeys([]key.Binding{other})
	if Claims(plain, "r") || !Claims(plain, "esc") {
		t.Errorf("a screen with no claim must get back on esc and no refresh on r")
	}
	if claimed := k.PanelKeys([]key.Binding{restore}); !Claims(claimed, "r") {
		t.Error("a screen claiming r keeps it")
	}
	if claimed := k.PanelKeys([]key.Binding{closeFlow}); slices.ContainsFunc(claimed, func(b key.Binding) bool { return b.Help().Desc == "back" }) {
		t.Error("a screen claiming esc must not also get the global back")
	}
	if got := k.PanelKeys([]key.Binding{other}); !slices.Equal(got[0].Keys(), other.Keys()) {
		t.Errorf("the screen's own keys must come first, got %v", got[0].Keys())
	}
}

func TestDescribedKeepsTheKeysAndChangesTheText(t *testing.T) {
	original := NewKeyMap().Refresh

	got := Described(original, "restore")

	if !slices.Equal(got.Keys(), original.Keys()) || got.Help().Key != original.Help().Key || got.Help().Desc != "restore" {
		t.Errorf("described = keys %v help %+v, want same keys and key text with desc restore", got.Keys(), got.Help())
	}
}
