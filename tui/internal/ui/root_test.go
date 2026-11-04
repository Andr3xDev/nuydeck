package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func press(t *testing.T, r Root, k tea.KeyPressMsg) (Root, tea.Cmd) {
	t.Helper()
	m, cmd := r.Update(k)
	return m.(Root), cmd
}

func TestRootQuitKeys(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{{Code: 'q', Text: "q"}, {Code: 'c', Mod: tea.ModCtrl}} {
		_, cmd := press(t, NewRoot("", false, Deps{}), k)
		if cmd == nil {
			t.Fatalf("%q: no command returned", k.String())
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%q: command did not quit", k.String())
		}
	}
}

func TestRootHelpToggleAndEsc(t *testing.T) {
	r := NewRoot("", false, Deps{})
	r, _ = press(t, r, tea.KeyPressMsg{Code: '?', Text: "?"})
	if !r.showHelp {
		t.Fatal("? did not open help")
	}
	r, _ = press(t, r, tea.KeyPressMsg{Code: tea.KeyEscape})
	if r.showHelp {
		t.Fatal("esc did not close help")
	}
	r, _ = press(t, r, tea.KeyPressMsg{Code: '?', Text: "?"})
	r, _ = press(t, r, tea.KeyPressMsg{Code: '?', Text: "?"})
	if r.showHelp {
		t.Fatal("second ? did not close help")
	}
}

func TestRootResizeKeepsSelectedSection(t *testing.T) {
	r := NewRoot("", true, Deps{})
	r, _ = press(t, r, tea.KeyPressMsg{Code: 'j', Text: "j"})
	r, _ = press(t, r, tea.KeyPressMsg{Code: tea.KeyDown})
	if r.selected != 2 {
		t.Fatalf("selected = %d, want 2", r.selected)
	}
	m, _ := r.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	r = m.(Root)
	if r.selected != 2 || r.width != 60 || r.height != 20 {
		t.Errorf("state after resize: selected=%d size=%dx%d", r.selected, r.width, r.height)
	}
}

func TestRootViewShowsDryRunBadgeOnEverySection(t *testing.T) {
	r := NewRoot("zenit", true, Deps{})
	m, _ := r.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	r = m.(Root)
	for i := range r.sections {
		r.selected = i
		v := r.View()
		if !v.AltScreen {
			t.Error("view must use the alt screen")
		}
		if !strings.Contains(ansi.Strip(v.Content), dryRunBadge) {
			t.Errorf("section %d: DRY-RUN badge missing", i)
		}
	}
	plain, _ := NewRoot("", false, Deps{}).Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if strings.Contains(ansi.Strip(plain.(Root).View().Content), dryRunBadge) {
		t.Error("badge shown without dry-run")
	}
}
