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
func testServices(deps Deps) *shell {
	return &shell{Deps: deps, ctx: context.Background(), th: ForVariant(""), keys: newKeyMap()}
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

	want := []string{"Dashboard", "Sync", "Backups", "Install", "Packages", "Config"}
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
	if got := sectionTitles(r); !slices.Equal(got, []string{"Manifest", "Config"}) {
		t.Errorf("sections with a paths.toml error = %v, want only Manifest and Config", got)
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

func TestRootReturnToDashboardSelectsItWithSidebarFocus(t *testing.T) {
	r := NewRoot("", false, Deps{})
	_, dashboard, _ := sectionOf[*Dashboard](r)
	_, install, _ := sectionOf[*Install](r)
	r.selectSection(install)
	r.focus = focusPanel

	r.Update(returnToDashboardMsg{})

	if r.selected != dashboard || r.focus != focusSidebar {
		t.Errorf("selected=%d focus=%v, want the dashboard (%d) with sidebar focus", r.selected, r.focus, dashboard)
	}
}

func TestRootSidebarListsTheNewSectionsAfterTheExistingOnes(t *testing.T) {
	r := NewRoot("", false, fullDeps())

	want := []string{"Dashboard", "Sync", "Backups", "Install", "Packages", "Services", "Env", "References", "Config"}
	if got := sectionTitles(r); !slices.Equal(got, want) {
		t.Errorf("sections = %v, want %v", got, want)
	}
}

func TestRootHidesTheNewSectionsWithoutTheirDeps(t *testing.T) {
	for _, title := range sectionTitles(NewRoot("", false, Deps{})) {
		if slices.Contains([]string{"Services", "Env", "References"}, title) {
			t.Errorf("section %s present without its Deps", title)
		}
	}
}

func TestRootManifestLoadErrorReplacesOnlyItsOwnSection(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Deps)
		section string
		file    string
	}{
		{"services.toml", func(d *Deps) { d.Services.Err = errors.New("entry 2: unknown scope") }, "Services", "services.toml"},
		{"references.toml", func(d *Deps) { d.References.Err = errors.New("entry 1: url must be https") }, "References", "references.toml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := fullDeps()
			tc.mutate(&deps)
			r := NewRoot("", false, deps)
			r.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

			for i, s := range r.sections {
				_, isError := s.screen.(errorScreen)
				if want := s.title == tc.section; isError != want {
					t.Errorf("section %s: error screen = %v, want %v", s.title, isError, want)
				}
				if s.title == tc.section {
					r.selected = i
				}
			}
			view := ansi.Strip(r.View().Content)
			if !strings.Contains(view, tc.file) {
				t.Errorf("error screen does not name %s:\n%s", tc.file, view)
			}
		})
	}
}

func TestRefreshIsListedForServicesAndEnvButNotReferences(t *testing.T) {
	root := NewRoot("", false, fullDeps())
	env, _, _ := sectionOf[*Env](root)
	feed(env, env.Enter())
	send(env, keyEnter)
	want := map[string]bool{"Services": true, "Env": true, "References": false}

	for i, s := range root.sections {
		wantR, ok := want[s.title]
		if !ok {
			continue
		}
		root.selected, root.focus = i, focusPanel
		if got := claims(root.focusedKeys(), "r"); got != wantR {
			t.Errorf("%s: r listed = %v, want %v", s.title, got, wantR)
		}
	}
}
