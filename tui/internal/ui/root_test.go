package ui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func press(t *testing.T, r *Root, k tea.KeyPressMsg) (*Root, tea.Cmd) {
	t.Helper()
	_, cmd := r.Update(k)
	return r, cmd
}

// testServices wires a screen under test the way NewRoot does.
func testServices(deps Deps) *services {
	return &services{Deps: deps, ctx: context.Background(), th: ForVariant(""), keys: DefaultKeyMap()}
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

func TestRootQuitsFromSidebarWhileInstallIsPastTheFirstPage(t *testing.T) {
	root, in := newTestRoot(&fakeInstaller{plan: onePlan()})
	toHardware(in)
	_, root.selected, _ = sectionOf[*Install](root)
	if in.page != pageHardware || root.panelFocus {
		t.Fatalf("page=%v panelFocus=%v, want the hardware page with sidebar focus", in.page, root.panelFocus)
	}

	_, cmd := press(t, root, keyPress('q'))

	if cmd == nil {
		t.Fatal("no command returned")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q with sidebar focus did not quit")
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
	r.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	r.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if r.selected != 2 || r.width != 60 || r.height != 20 {
		t.Errorf("state after resize: selected=%d size=%dx%d", r.selected, r.width, r.height)
	}
}

func sectionTitles(r *Root) []string {
	titles := make([]string, len(r.sections))
	for i, s := range r.sections {
		titles[i] = s.title
	}
	return titles
}

func TestRootSidebarOrderKeepsSyncSection(t *testing.T) {
	r := NewRoot("", false, Deps{})

	want := []string{"Dashboard", "Sync", "Backups", "Install", "Packages"}
	if got := sectionTitles(r); !slices.Equal(got, want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	if _, at, ok := sectionOf[*Sync](r); !ok || at != 1 {
		t.Errorf("Sync section at %d (found %v), want 1", at, ok)
	}
}

func TestRootCatalogErrorReplacesOnlyInstallAndPackages(t *testing.T) {
	r := NewRoot("", false, Deps{CatalogError: errors.New("packages.toml:3: bad source")})

	for _, s := range r.sections {
		_, isError := s.screen.(errorScreen)
		if want := s.title == "Install" || s.title == "Packages"; isError != want {
			t.Errorf("section %s: error screen = %v, want %v", s.title, isError, want)
		}
	}

	r = NewRoot("", false, Deps{ManifestError: errors.New("paths.toml:1: bad"), CatalogError: errors.New("packages.toml:3: bad source")})
	if got := sectionTitles(r); !slices.Equal(got, []string{"Manifest"}) {
		t.Errorf("sections with a paths.toml error = %v, want only Manifest", got)
	}
}

func TestRootViewShowsDryRunBadgeOnEverySection(t *testing.T) {
	r := NewRoot("zenit", true, Deps{})
	r.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
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
	plain := NewRoot("", false, Deps{})
	plain.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if strings.Contains(ansi.Strip(plain.View().Content), dryRunBadge) {
		t.Error("badge shown without dry-run")
	}
}
