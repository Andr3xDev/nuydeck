package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// probeScreen records what the shell delivers to it.
type probeScreen struct {
	keys     []key.Binding
	captures bool
	received []string
	entered  int
	// broadcasts are the non-key messages Root delivered.
	broadcasts []tea.Msg
}

func (p *probeScreen) Title() string  { return "Probe" }
func (p *probeScreen) Enter() tea.Cmd { p.entered++; return nil }
func (p *probeScreen) View(int, int) string {
	return ""
}
func (p *probeScreen) Keys() []key.Binding { return p.keys }
func (p *probeScreen) CapturesText() bool  { return p.captures }
func (p *probeScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		p.received = append(p.received, k.String())
	} else {
		p.broadcasts = append(p.broadcasts, msg)
	}
	return p, nil
}

func bindingFor(keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], "probe "+keys[0]))
}

// probeRoot puts the probe in the first section, a second probe after it, and starts with the given focus.
func probeRoot(p *probeScreen, f focus) (*Root, *probeScreen) {
	other := &probeScreen{}
	r := NewRoot("", false, Deps{})
	r.sections = []section{{title: "Probe", screen: p}, {title: "Other", screen: other}}
	r.selected, r.focus = 0, f
	return r, other
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func probeKeys() []key.Binding {
	return []key.Binding{bindingFor("up", "k"), bindingFor("down", "j"), bindingFor("enter", "right", "l"), bindingFor("x")}
}

func TestRootStartsWithSidebarFocus(t *testing.T) {
	if r := NewRoot("", false, Deps{}); r.focus != focusSidebar {
		t.Errorf("focus = %v, want focusSidebar", r.focus)
	}
}

func TestFocusTable(t *testing.T) {
	tests := []struct {
		name          string
		focus         focus
		key           tea.KeyPressMsg
		wantFocus     focus
		wantSelected  int
		wantDelivered bool
	}{
		{"tab to panel", focusSidebar, keyTab, focusPanel, 0, false},
		{"tab to sidebar", focusPanel, keyTab, focusSidebar, 0, false},
		{"enter opens panel without a screen action", focusSidebar, keyEnter, focusPanel, 0, false},
		{"right opens panel without a screen action", focusSidebar, keyRight, focusPanel, 0, false},
		{"l opens panel without a screen action", focusSidebar, keyPress('l'), focusPanel, 0, false},
		{"enter reaches the screen in the panel", focusPanel, keyEnter, focusPanel, 0, true},
		{"right reaches the screen in the panel", focusPanel, keyRight, focusPanel, 0, true},
		{"down moves the selection at the sidebar", focusSidebar, keyDown, focusSidebar, 1, false},
		{"j moves the selection at the sidebar", focusSidebar, keyPress('j'), focusSidebar, 1, false},
		{"down reaches the screen in the panel", focusPanel, keyDown, focusPanel, 0, true},
		{"j reaches the screen in the panel", focusPanel, keyPress('j'), focusPanel, 0, true},
		{"panel key is not delivered at the sidebar", focusSidebar, keyPress('x'), focusSidebar, 0, false},
		{"panel key is delivered in the panel", focusPanel, keyPress('x'), focusPanel, 0, true},
		{"key outside Keys is not delivered", focusPanel, keyPress('z'), focusPanel, 0, false},
		{"esc at the sidebar does nothing", focusSidebar, keyEsc, focusSidebar, 0, false},
		{"esc in the panel returns to the sidebar", focusPanel, keyEsc, focusSidebar, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := &probeScreen{keys: probeKeys()}
			r, _ := probeRoot(probe, tt.focus)

			press(t, r, tt.key)

			if r.focus != tt.wantFocus || r.selected != tt.wantSelected {
				t.Errorf("focus=%v selected=%d, want focus=%v selected=%d", r.focus, r.selected, tt.wantFocus, tt.wantSelected)
			}
			if delivered := len(probe.received) > 0; delivered != tt.wantDelivered {
				t.Errorf("delivered = %v (%v), want %v", delivered, probe.received, tt.wantDelivered)
			}
		})
	}
}

func TestBordersFollowFocus(t *testing.T) {
	r := sizedRoot(Deps{}, 80, 24)
	th := r.svc.th
	// First row: the sidebar's right border directly followed by the panel's top-left corner.
	seam := func(sidebar, panel lipgloss.Style) string { return sidebar.Render("│") + panel.Render("╭─") }

	if top := strings.Split(r.View().Content, "\n")[0]; !strings.Contains(top, seam(th.borderFocus, th.border)) {
		t.Errorf("sidebar focus: want accent sidebar border and muted panel border, got %q", top)
	}
	r.focus = focusPanel
	if top := strings.Split(r.View().Content, "\n")[0]; !strings.Contains(top, seam(th.border, th.borderFocus)) {
		t.Errorf("panel focus: want muted sidebar border and accent panel border, got %q", top)
	}
	r.focus = focusSidebar
	r.run = &run{}
	if !r.panelFocused() {
		t.Error("an active run pane must count as focused")
	}
}

func TestSidebarIgnoresTheJumpKeys(t *testing.T) {
	// Each key starts where a jump would land elsewhere: J has room below row 0, K above row 1.
	for k, start := range map[tea.KeyPressMsg]int{keyPress('J'): 0, keyPress('K'): 1} {
		probe := &probeScreen{}
		r, other := probeRoot(probe, focusSidebar)
		r.selected = start

		press(t, r, k)

		if r.selected != start || probe.entered != 0 || other.entered != 0 {
			t.Errorf("%q: selected=%d entered=%d/%d, want the selection at %d and nothing entered", k.String(), r.selected, probe.entered, other.entered, start)
		}
	}
}

func TestSidebarMoveEntersTheNewScreen(t *testing.T) {
	r, other := probeRoot(&probeScreen{}, focusSidebar)

	press(t, r, keyDown)
	press(t, r, keyUp)

	if other.entered != 1 {
		t.Errorf("Enter calls on the newly selected screen = %d, want 1", other.entered)
	}
}

func TestEscIsDeliveredWhenTheScreenClaimsIt(t *testing.T) {
	probe := &probeScreen{keys: []key.Binding{bindingFor("esc")}}
	r, _ := probeRoot(probe, focusPanel)

	press(t, r, keyEsc)

	if r.focus != focusPanel || len(probe.received) != 1 {
		t.Errorf("focus=%v received=%v, want esc delivered and focus kept", r.focus, probe.received)
	}
}

// Only screens that list r refresh; for the others the keybar shows no r, so the key does nothing.
func TestUnclaimedRDoesNothing(t *testing.T) {
	probe := &probeScreen{keys: probeKeys()}
	r, _ := probeRoot(probe, focusPanel)

	press(t, r, keyPress('r'))

	if probe.entered != 0 || len(probe.received) != 0 {
		t.Errorf("Enter calls = %d, received = %v, want r ignored", probe.entered, probe.received)
	}
}

func TestClaimedRIsDeliveredAndNotAlsoRefreshed(t *testing.T) {
	probe := &probeScreen{keys: append(probeKeys(), bindingFor("r"))}
	r, _ := probeRoot(probe, focusPanel)

	press(t, r, keyPress('r'))

	if probe.entered != 0 || len(probe.received) != 1 {
		t.Errorf("entered=%d received=%v, want r delivered once and no refresh", probe.entered, probe.received)
	}
}

func TestHelpOpensFromBothFocuses(t *testing.T) {
	for _, f := range []focus{focusSidebar, focusPanel} {
		r, _ := probeRoot(&probeScreen{keys: probeKeys()}, f)
		press(t, r, keyPress('?'))
		if !r.showHelp {
			t.Errorf("focus %v: ? did not open help", f)
		}
	}
}

func TestQuitKeysFromBothFocuses(t *testing.T) {
	for _, f := range []focus{focusSidebar, focusPanel} {
		for _, k := range []tea.KeyPressMsg{keyPress('q'), keyCtrlC} {
			r, _ := probeRoot(&probeScreen{keys: probeKeys()}, f)
			if _, cmd := press(t, r, k); !isQuit(cmd) {
				t.Errorf("focus %v: %q did not quit", f, k.String())
			}
		}
	}
}

func TestTextCaptureReceivesShellKeysInThePanel(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{keyPress('q'), keyPress('?')} {
		probe := &probeScreen{captures: true}
		r, _ := probeRoot(probe, focusPanel)

		_, cmd := press(t, r, k)

		if isQuit(cmd) || r.showHelp || len(probe.received) != 1 {
			t.Errorf("%q: quit=%v help=%v received=%v, want only delivery to the text screen", k.String(), isQuit(cmd), r.showHelp, probe.received)
		}
	}
}

func TestTextCaptureNeverBlocksCtrlC(t *testing.T) {
	r, _ := probeRoot(&probeScreen{captures: true}, focusPanel)

	if _, cmd := press(t, r, keyCtrlC); !isQuit(cmd) {
		t.Error("ctrl+c did not quit while a screen captures text")
	}
}

func TestTextCaptureDoesNotApplyAtTheSidebar(t *testing.T) {
	probe := &probeScreen{captures: true}
	r, _ := probeRoot(probe, focusSidebar)

	if _, cmd := press(t, r, keyPress('q')); !isQuit(cmd) || len(probe.received) != 0 {
		t.Errorf("q at the sidebar: quit=%v received=%v, want quit and no delivery", isQuit(cmd), probe.received)
	}
}

func TestEnterAtSidebarDoesNotStartTheInstallWizard(t *testing.T) {
	f := &fakeInstaller{plan: onePlan()}
	root, in := newTestRoot(f)
	_, root.selected, _ = sectionOf[*Install](root)

	press(t, root, keyEnter)

	if root.focus != focusPanel || in.page != pageProfile || len(f.drafted) != 0 {
		t.Errorf("focus=%v page=%v plans=%d, want panel focus, still the profile page and no plan", root.focus, in.page, len(f.drafted))
	}
}

func TestEscGoesBackAndQNeverDoes(t *testing.T) {
	setup := func() (*Root, *Install) {
		root, in := newTestRoot(&fakeInstaller{plan: onePlan()})
		toHardware(in)
		_, root.selected, _ = sectionOf[*Install](root)
		root.focus = focusPanel
		return root, in
	}

	root, in := setup()
	press(t, root, keyEsc)
	if in.page != pageProfile || root.focus != focusPanel {
		t.Errorf("esc: page=%v focus=%v, want one step back inside the wizard", in.page, root.focus)
	}

	root, in = setup()
	_, cmd := press(t, root, keyPress('q'))
	if !isQuit(cmd) || in.page != pageHardware {
		t.Errorf("q: quit=%v page=%v, want quit and no back step", isQuit(cmd), in.page)
	}
}
