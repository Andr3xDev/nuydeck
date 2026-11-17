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
	for _, s := range NewRoot("", false, Deps{}).sections {
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

func TestPanelKeysFollowTheClaimRules(t *testing.T) {
	k := newKeyMap()
	restore := described(key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restore")), "restore")
	other := key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "other"))
	closeFlow := key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))

	plain := k.panelKeys([]key.Binding{other})
	if !claims(plain, "r") || !claims(plain, "esc") {
		t.Errorf("a screen with no claim must get refresh on r and back on esc")
	}
	if claimed := k.panelKeys([]key.Binding{restore}); slices.ContainsFunc(claimed, func(b key.Binding) bool { return b.Help().Desc == "refresh" }) {
		t.Error("a screen claiming r must not also get the global refresh")
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
	if s.phase != phaseBackupDialog {
		t.Fatalf("phase = %v, want the backup dialog", s.phase)
	}

	for _, outside := range []tea.KeyPressMsg{keyPress('p'), keyPress('P'), keyPress('a'), keyPress(tea.KeySpace)} {
		_, cmd := press(t, root, outside)
		if cmd != nil || s.phase != phaseBackupDialog {
			t.Errorf("%q: cmd=%v phase=%v, want it ignored by the dialog", outside.String(), cmd != nil, s.phase)
		}
	}
}
