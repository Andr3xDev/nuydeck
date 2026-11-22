package ui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
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
	k := kit.NewKeyMap()
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
	keyLists["run pane"] = newRun(runRequest{}, func() {}).keys(kit.NewKeyMap())

	move := kit.NewKeyMap().Move()
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

	if !kit.Claims(in.Keys(), "ctrl+s") {
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
