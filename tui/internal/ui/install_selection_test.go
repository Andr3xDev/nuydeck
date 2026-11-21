package ui

import (
	"context"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

const dependenciesGroup = "Dependencies"

func selectionCatalog() []manifest.Package {
	return []manifest.Package{
		{Name: "vim", Group: "editor", Profiles: []string{manifest.ProfileMinimal}},
		{Name: "nano", Group: "editor"},
		{Name: "libfoo", Group: "vcs"},
		{Name: "git", Group: "vcs"},
	}
}

// rowLabels shows headers as [group] and packages by name.
func rowLabels(rows []selectionRow) []string {
	labels := make([]string, len(rows))
	for i, r := range rows {
		labels[i] = r.name
		if r.name == "" {
			labels[i] = "[" + r.group + "]"
		}
	}
	return labels
}

func TestBuildRowsPartitionsDependenciesLastAndSortsAlphabetically(t *testing.T) {
	rows := buildRows(selectionCatalog(), map[string]bool{"libfoo": true, "nano": true})

	want := []string{"[editor]", "vim", "[vcs]", "git", "[" + dependenciesGroup + "]", "libfoo", "nano"}
	if got := rowLabels(rows); !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestBuildRowsHeaderNamesHoldOnlyItsOwnMembers(t *testing.T) {
	rows := buildRows(selectionCatalog(), map[string]bool{"libfoo": true, "nano": true})

	got := map[string][]string{}
	for _, r := range rows {
		if r.name == "" {
			got[r.group] = r.names
		}
	}
	want := map[string][]string{"editor": {"vim"}, "vcs": {"git"}, dependenciesGroup: {"libfoo", "nano"}}
	for group, names := range want {
		if !slices.Equal(got[group], names) {
			t.Errorf("header %q names = %v, want %v", group, got[group], names)
		}
	}
}

func TestBuildRowsShowsNoDependenciesGroupWithoutDependencies(t *testing.T) {
	for name, set := range map[string]map[string]bool{"probe failed": nil, "probe found nothing": {}} {
		t.Run(name, func(t *testing.T) {
			want := []string{"[editor]", "nano", "vim", "[vcs]", "git", "libfoo"}
			if got := rowLabels(buildRows(selectionCatalog(), set)); !slices.Equal(got, want) {
				t.Errorf("rows = %v, want %v", got, want)
			}
		})
	}
}

// toSelection leaves the wizard on the selection page with the custom profile.
func toSelection(t *testing.T) *Install {
	t.Helper()
	f := &fakeInstaller{plan: onePlan()}
	root := NewRoot("", false, Deps{Install: InstallDeps{
		Plan:         f.Plan,
		Run:          f.Run,
		Catalog:      selectionCatalog(),
		Dependencies: func(context.Context) (map[string]bool, error) { return nil, nil },
	}})
	in, _, _ := sectionOf[*Install](root)
	tap(in, specialKey(tea.KeyDown))
	tap(in, specialKey(tea.KeyDown))
	settle(in, tap(in, specialKey(tea.KeyEnter)))
	settle(in, tap(in, specialKey(tea.KeyEnter)))
	if in.page != pageSelection {
		t.Fatalf("page = %v, want the selection", in.page)
	}
	return in
}

func TestInstallLateDependencyProbeKeepsSelectionsAndTheCursorRow(t *testing.T) {
	in := toSelection(t)
	cursorTo(t, in, func(r selectionRow) bool { return r.name == "git" })
	tap(in, keyPress(tea.KeySpace))

	in.Update(dependenciesMsg{set: map[string]bool{"git": true}})

	if !in.selected["git"] || !in.selected["vim"] || in.selected["nano"] {
		t.Errorf("selected = %v, want the toggles kept across the rebuild", in.selected)
	}
	if row := in.rows[in.cursor]; row.name != "git" {
		t.Errorf("cursor row = %+v, want it to follow git into the Dependencies group", row)
	}
}

func TestInstallDependenciesHeaderTogglesOnlyItsMembers(t *testing.T) {
	in := toSelection(t)
	in.Update(dependenciesMsg{set: map[string]bool{"libfoo": true, "nano": true}})
	cursorTo(t, in, func(r selectionRow) bool { return r.name == "" && r.group == dependenciesGroup })

	tap(in, keyPress(tea.KeySpace))

	if !in.selected["libfoo"] || !in.selected["nano"] || in.selected["git"] || !in.selected["vim"] {
		t.Errorf("selected = %v, want only libfoo and nano switched on", in.selected)
	}
}

func TestInstallSelectAllSelectsEverythingThenClearsWhenAllAreSelected(t *testing.T) {
	in := toSelection(t)

	tap(in, keyPress('a'))
	all := slices.Equal(selectedNames(in), []string{"git", "libfoo", "nano", "vim"})
	tap(in, keyPress('a'))

	if !all || len(selectedNames(in)) != 0 {
		t.Errorf("after the first a all selected = %v; after the second selected = %v, want everything then nothing", all, selectedNames(in))
	}
}

// cursorTo moves the cursor down until the row matches.
func cursorTo(t *testing.T, in *Install, match func(selectionRow) bool) {
	t.Helper()
	for range in.rows {
		if match(in.rows[in.cursor]) {
			return
		}
		tap(in, specialKey(tea.KeyDown))
	}
	t.Fatalf("no row matched among %v", rowLabels(in.rows))
}

func selectedNames(in *Install) []string {
	var names []string
	for name, on := range in.selected {
		if on {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}
