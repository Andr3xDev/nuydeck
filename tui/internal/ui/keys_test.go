package ui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
)

// keyedStages returns every screen stage worth checking: each default section plus the stages that change Keys().
func keyedStages(t *testing.T) map[string]screen {
	t.Helper()
	stages := map[string]screen{}
	for _, s := range NewRoot("", false, fullDeps()).sections {
		stages["section "+s.title] = s.screen
	}

	_, hardware := newTestRoot(&fakeInstaller{plan: onePlan()})
	toHardware(hardware)
	stages["install hardware"] = hardware

	_, secrets := newTestRoot(&fakeInstaller{plan: onePlan()}, "TOKEN")
	toHardware(secrets)
	settle(secrets, tap(secrets, specialKey(tea.KeyEnter)))
	stages["install secrets"] = secrets

	item := planItem("a", false)
	plan := cfgsync.SyncPlan{Items: []cfgsync.PlanItem{item}, Selection: cfgsync.Selection{item.Key(): true}}
	preview := newPreview(&fakeSyncDeps{plan: plan, needsBackup: true}, false)
	stages["sync preview"] = preview
	dialog := newPreview(&fakeSyncDeps{plan: plan, needsBackup: true}, false)
	dialog.Update(keyPress(tea.KeyEnter))
	stages["sync dialog"] = dialog
	return stages
}

func TestEveryScreenBindingHasHelpText(t *testing.T) {
	for name, s := range keyedStages(t) {
		for _, b := range s.Keys() {
			if len(b.Keys()) == 0 || b.Help().Key == "" || b.Help().Desc == "" {
				t.Errorf("%s: binding %v has help %+v, want a key and a description", name, b.Keys(), b.Help())
			}
		}
	}
}

func TestNoScreenBindingCollidesWithAGlobalKey(t *testing.T) {
	k := newKeyMap()
	global := slices.Concat(k.Quit.Keys(), k.Help.Keys(), k.Focus.Keys())

	for name, s := range keyedStages(t) {
		for _, b := range s.Keys() {
			for _, pressed := range b.Keys() {
				if slices.Contains(global, pressed) {
					t.Errorf("%s: binding %q collides with a global key", name, pressed)
				}
			}
		}
	}
}

// Every list or scrolling stage must advertise J/K next to up/down; stages without a cursor are skipped with a reason.
func TestKeyListsOfListStagesIncludeMove(t *testing.T) {
	skipped := map[string]string{
		"section Config":  "read-only values, no cursor",
		"section Sync":    "pull/push launcher; its list is the preview stage",
		"section Env":     "no list until a source is chosen",
		"sync dialog":     "yes/no question, no list",
		"install secrets": "text input, no list",
	}
	keyLists := map[string][]key.Binding{}
	for name, s := range keyedStages(t) {
		keyLists[name] = s.Keys()
	}
	keyLists["install selection"] = toSelection(t).Keys()
	_, summary := newTestRoot(&fakeInstaller{plan: onePlan()})
	toSummary(summary)
	keyLists["install summary"] = summary.Keys()
	h := newTestFlow(t, withExtra("", ""))
	planned(h.f)
	keyLists["flow body"] = h.f.Keys()
	keyLists["run pane"] = newRun(runRequest{}, func() {}).keys(newKeyMap())

	move := newKeyMap().move()
	for name, keys := range keyLists {
		if _, ok := skipped[name]; ok {
			continue
		}
		for _, want := range move {
			if !slices.ContainsFunc(keys, func(b key.Binding) bool { return slices.Equal(b.Keys(), want.Keys()) }) {
				t.Errorf("%s: Keys() lacks %v", name, want.Keys())
			}
		}
	}
}

func TestKeyMapStep(t *testing.T) {
	k := newKeyMap()
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
		if got := k.step(c.msg); got != c.want {
			t.Errorf("%s: step = %d, want %d", c.name, got, c.want)
		}
		wantArrow := c.want
		if wantArrow != 1 && wantArrow != -1 {
			wantArrow = 0
		}
		if got := k.arrow(c.msg); got != wantArrow {
			t.Errorf("%s: arrow = %d, want %d", c.name, got, wantArrow)
		}
	}
	if got, want := k.move(), []key.Binding{k.Up, k.Down, k.JumpUp, k.JumpDown}; len(got) != len(want) {
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
		if got := moveCursor(c.cursor, c.delta, c.n); got != c.want {
			t.Errorf("%s: moveCursor(%d, %d, %d) = %d, want %d", c.name, c.cursor, c.delta, c.n, got, c.want)
		}
	}
}

func TestPanelKeysFollowTheClaimRules(t *testing.T) {
	k := newKeyMap()
	restore := described(key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restore")), "restore")
	other := key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "other"))
	closeFlow := key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))

	plain := k.panelKeys([]key.Binding{other})
	if claims(plain, "r") || !claims(plain, "esc") {
		t.Errorf("a screen with no claim must get back on esc and no refresh on r")
	}
	if claimed := k.panelKeys([]key.Binding{restore}); !claims(claimed, "r") {
		t.Error("a screen claiming r keeps it")
	}
	if claimed := k.panelKeys([]key.Binding{closeFlow}); slices.ContainsFunc(claimed, func(b key.Binding) bool { return b.Help().Desc == "back" }) {
		t.Error("a screen claiming esc must not also get the global back")
	}
	if got := k.panelKeys([]key.Binding{other}); !slices.Equal(got[0].Keys(), other.Keys()) {
		t.Errorf("the screen's own keys must come first, got %v", got[0].Keys())
	}
}

func TestDescribedKeepsTheKeysAndChangesTheText(t *testing.T) {
	original := newKeyMap().Refresh

	got := described(original, "restore")

	if !slices.Equal(got.Keys(), original.Keys()) || got.Help().Key != original.Help().Key || got.Help().Desc != "restore" {
		t.Errorf("described = keys %v help %+v, want same keys and key text with desc restore", got.Keys(), got.Help())
	}
}

func TestHelpListsEveryScreenBinding(t *testing.T) {
	root, _ := newTestRoot(&fakeInstaller{plan: onePlan()})
	root.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	for i, s := range root.sections {
		if _, isError := s.screen.(errorScreen); isError {
			continue
		}
		root.selected, root.focus, root.showHelp = i, focusPanel, false
		press(t, root, keyPress('?'))
		view := ansi.Strip(root.View().Content)
		for _, b := range root.sections[i].screen.Keys() {
			if !strings.Contains(view, b.Help().Key) || !strings.Contains(view, b.Help().Desc) {
				t.Errorf("section %s: help does not list %q (%s)", s.title, b.Help().Key, b.Help().Desc)
			}
		}
	}
}

func TestSecretsSkipKeyIsABindingAndVisible(t *testing.T) {
	root, in := newTestRoot(&fakeInstaller{plan: onePlan()}, "TOKEN")
	toHardware(in)
	settle(in, tap(in, specialKey(tea.KeyEnter)))
	if in.page != pageSecrets {
		t.Fatalf("page = %v, want the secrets page", in.page)
	}
	_, root.selected, _ = sectionOf[*Install](root)
	root.focus = focusPanel
	root.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	if !claims(in.Keys(), "ctrl+s") {
		t.Error("ctrl+s is not a binding of the secrets page")
	}
	if !strings.Contains(ansi.Strip(root.View().Content), "ctrl+s") {
		t.Error("ctrl+s is not visible in the keybar or the page")
	}
}

func TestKeyOutsideTheStageKeysIsIgnored(t *testing.T) {
	item := planItem("a", false)
	plan := cfgsync.SyncPlan{Items: []cfgsync.PlanItem{item}, Selection: cfgsync.Selection{item.Key(): true}}
	root := NewRoot("", false, Deps{})
	s := newPreview(&fakeSyncDeps{plan: plan, needsBackup: true}, false)
	_, at, _ := sectionOf[*Sync](root)
	root.sections[at].screen = s
	root.selected, root.focus = at, focusPanel
	press(t, root, keyEnter)
	if s.flow.stage != stageOffer {
		t.Fatalf("stage = %v, want the backup offer", s.flow.stage)
	}

	for _, outside := range []tea.KeyPressMsg{keyPress('p'), keyPress('P'), keyPress('a'), keyPress(tea.KeySpace)} {
		_, cmd := press(t, root, outside)
		if cmd != nil || s.flow.stage != stageOffer {
			t.Errorf("%q: cmd=%v stage=%v, want it ignored by the offer", outside.String(), cmd != nil, s.flow.stage)
		}
	}
}
