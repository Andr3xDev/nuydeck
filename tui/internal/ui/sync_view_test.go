package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/drift"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
)

func otherEntry() manifest.ResolvedEntry {
	return manifest.ResolvedEntry{Entry: manifest.Entry{Root: manifest.RootConfig, Path: "kitty"}}
}

// Dashboard p and P hand the entry to the mode popup; the Sync screen's own keys cover every entry.
func TestSyncScopeFromTheDashboardPlansOnlyThatEntryAndNamesItInTheTitle(t *testing.T) {
	f := newModeDeps()
	deps := f.deps()
	deps.Entries = []manifest.ResolvedEntry{hyprEntry(), otherEntry()}
	var mu sync.Mutex
	var recorded [][]manifest.ResolvedEntry
	planned := func() [][]manifest.ResolvedEntry {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(recorded)
	}
	deps.PlanHard = func(_ context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (cfgsync.HardPlan, error) {
		mu.Lock()
		recorded = append(recorded, entries)
		mu.Unlock()
		return f.PlanHard(context.Background(), dir, entries)
	}
	root := NewRoot("", false, deps)
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	d := newDriver(t, root)
	s, _, _ := sectionOf[*Sync](root)

	_, cmd := root.Update(startSyncMsg{direction: plan.Push, entries: []manifest.ResolvedEntry{hyprEntry()}})
	if cmd != nil || !strings.Contains(d.view(), "config:hypr") || !strings.Contains(d.view(), hardModeText) {
		t.Fatalf("the dashboard request must open the mode popup naming the entry:\n%s", d.view())
	}
	d.press(keyDown)
	d.press(keyEnter)
	d.must(t, "the Hard list", inStage(s, stagePreview))
	if got := planned(); len(got) != 1 || len(got[0]) != 1 || got[0][0].Path != "hypr" {
		t.Errorf("hard planned %v, want only hypr", got)
	}
	if !strings.Contains(s.flow.src.Title, "config:hypr") {
		t.Errorf("title = %q, want the scope in it", s.flow.src.Title)
	}

	d.press(keyEsc)
	d.press(keyPress('P'))
	d.press(keyDown)
	d.press(keyEnter)
	d.must(t, "the Hard list again", func() bool { return len(planned()) == 2 && inStage(s, stagePreview)() })
	if got := planned(); len(got[1]) != 2 || s.flow.src.Title != directionLabel(plan.Push) {
		t.Errorf("the Sync screen's own P planned %d entries under %q, want all 2 under the plain label", len(got[1]), s.flow.src.Title)
	}
}

func TestSyncVBeforeTheSidesLoadOpensTheDiffWhenTheyArrive(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old\n", "new\n")
	f.gate = make(chan struct{})
	d := resolverRoot(t, f)
	d.mustShow(t, "loading")

	d.press(keyPress('v'))
	d.settle()
	if strings.Contains(d.view(), "destination") {
		t.Fatal("the diff opened before the sides were read")
	}

	close(f.gate)
	d.mustShow(t, "destination")
}

func TestSyncVThenMovingAwayDoesNotOpenALaterDiff(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update), resolverItem("b.conf", plan.Update))
	f.withText("a.conf", "old\n", "new\n")
	f.withText("b.conf", "old\n", "new\n")
	f.gate = make(chan struct{})
	d := resolverRoot(t, f)
	d.mustShow(t, "loading")

	d.press(keyPress('v'))
	d.press(keyPress('j'))
	close(f.gate)
	d.mustLoad(t)
	d.settle()

	if strings.Contains(d.view(), "destination") {
		t.Errorf("a diff asked for on a.conf opened after the cursor moved to b.conf:\n%s", d.view())
	}
}

// hardListRoot opens the Hard list of the resolver deps' plan, selecting every item as PlanHard does.
func hardListRoot(t *testing.T, f *resolverDeps) (*driver, *Sync) {
	t.Helper()
	deps := f.deps()
	deps.PlanHard = func(_ context.Context, dir plan.Direction, _ []manifest.ResolvedEntry) (cfgsync.HardPlan, error) {
		sel := cfgsync.Selection{}
		for _, it := range f.plan.Items {
			sel[it.Key()] = true
		}
		return cfgsync.HardPlan{SyncPlan: f.plan, Selection: sel, Confirm: "Hard reinstall: repo -> PC"}, nil
	}
	root := NewRoot("", false, deps)
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	s, at, _ := sectionOf[*Sync](root)
	root.selected, root.focus = at, focusPanel
	d := newDriver(t, root)
	d.press(keyPress('P'))
	d.press(keyDown)
	d.press(keyEnter)
	d.must(t, "the Hard list", inStage(s, stagePreview))
	return d, s
}

func TestSyncHardListShowsItemsOpensTheDiffAndStillConfirms(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update), resolverItem("extra", plan.Delete))
	f.withText("a.conf", "old\n", "new\n")
	f.withText("extra", "gone\n", "")
	d, s := hardListRoot(t, f)

	view := d.view()
	for _, want := range []string{"a.conf", "extra", "delete", "[x] take"} {
		if !strings.Contains(view, want) {
			t.Errorf("hard list lacks %q:\n%s", want, view)
		}
	}
	d.press(keyPress(tea.KeySpace))
	d.press(keyPress('e'))
	d.settle()
	if f.editCalls() != 0 || !strings.Contains(rowOf(d.view(), "a.conf"), "[x] take") {
		t.Fatal("a hard list must not toggle or edit items")
	}

	d.mustLoad(t)
	d.press(keyPress('v'))
	d.mustShow(t, "destination")
	d.press(keyEsc)
	d.must(t, "the list back", inStage(s, stagePreview))

	d.press(keyEnter)
	d.must(t, "the confirm", inStage(s, stageConfirm))
	if f.applyCalls.Load() != 0 || !strings.Contains(d.view(), "Hard reinstall") {
		t.Errorf("the hard confirm must show before anything is written:\n%s", d.view())
	}
}

// driftViewRoot is a Dashboard on a drifted entry (or an in-sync one) backed by the resolver deps' plan.
func driftViewRoot(t *testing.T, f *resolverDeps, result drift.DriftResult) (*driver, *Dashboard, *Sync) {
	t.Helper()
	deps := f.deps()
	deps.Entries = []manifest.ResolvedEntry{hyprEntry()}
	root := NewRoot("", false, deps)
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	dash, _, _ := sectionOf[*Dashboard](root)
	s, _, _ := sectionOf[*Sync](root)
	root.focus = focusPanel
	result.Entry = hyprEntry()
	dash.results[hyprEntry().Entry] = result
	return newDriver(t, root), dash, s
}

func TestDashboardVOpensAReadOnlyDriftViewThatNeverApplies(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update), resolverItem("new.conf", plan.Create), resolverItem("extra", plan.Delete))
	f.withText("a.conf", "old\n", "new\n")
	d, _, s := driftViewRoot(t, f, drift.DriftResult{State: drift.DriftDrifted})

	d.press(keyPress('v'))
	d.must(t, "the drift list", inStage(s, stagePreview))
	view := d.view()
	for _, want := range []string{"drift · config:hypr", "update home:a.conf", "create home:new.conf", "delete home:extra"} {
		if !strings.Contains(view, want) {
			t.Errorf("drift view lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "[x]") || strings.Contains(view, "[ ]") {
		t.Errorf("drift view shows a choice marker:\n%s", view)
	}

	d.press(keyPress(tea.KeySpace))
	d.press(keyEnter)
	d.press(keyPress('y'))
	d.settle()
	if s.flow.stage != stagePreview || d.root.run != nil {
		t.Fatalf("the drift view must not confirm or run: stage=%v", s.flow.stage)
	}

	d.mustLoad(t)
	d.press(keyPress('v'))
	d.mustShow(t, "destination")
	d.press(keyEsc)
	d.must(t, "the list back", inStage(s, stagePreview))

	d.press(keyEsc)
	d.must(t, "the dashboard", func() bool { return strings.Contains(d.view(), "Last sync") })
	if f.applyCalls.Load() != 0 {
		t.Error("a drift view must never apply")
	}
}

func TestDashboardVOnACleanRowSaysThereIsNoDrift(t *testing.T) {
	d, dash, s := driftViewRoot(t, newResolverDeps(plan.Push), drift.DriftResult{State: drift.DriftInSync})

	d.press(keyPress('v'))
	d.settle()

	if s.flow.active() || !strings.Contains(d.view(), noDriftText) || len(dash.Notices()) != 1 {
		t.Errorf("v on an in-sync row must only show the notice:\n%s", d.view())
	}
}

func TestDashboardVOnAFailedRowNamesTheDriftError(t *testing.T) {
	d, dash, s := driftViewRoot(t, newResolverDeps(plan.Push), drift.DriftResult{State: drift.DriftError, Err: errors.New("boom")})

	d.press(keyPress('v'))
	d.settle()

	if s.flow.active() || !strings.Contains(d.view(), "drift check failed: boom") || len(dash.Notices()) != 1 {
		t.Errorf("v on an errored row must show the drift error as a notice:\n%s", d.view())
	}
}

func TestDriftViewExplainsAnEmptyPlanOrAPlannerError(t *testing.T) {
	planErr := newResolverDeps(plan.Push)
	planErr.planErr = errors.New("source missing")
	tests := []struct {
		name   string
		f      *resolverDeps
		result drift.DriftResult
		want   string
	}{
		{"only on this PC", planErr, drift.DriftResult{State: drift.DriftDrifted, Missing: drift.MissingInRepo}, "only on this PC: not in the repo"},
		{"not on this PC", newResolverDeps(plan.Push), drift.DriftResult{State: drift.DriftDrifted, Missing: drift.MissingOnPC}, "not on this PC yet"},
		{"excluded files only", newResolverDeps(plan.Push), drift.DriftResult{State: drift.DriftDrifted}, "drift is only in excluded files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _, _ := driftViewRoot(t, tt.f, tt.result)
			d.press(keyPress('v'))
			d.mustShow(t, tt.want)
			if strings.Contains(d.view(), "source missing") || strings.Contains(d.view(), "Nothing to sync.") {
				t.Errorf("drift view leaks the planner wording:\n%s", d.view())
			}
		})
	}
}

func TestSyncDiffViewOfLinksShowsBothTargetsAndWhetherTheyResolve(t *testing.T) {
	linkItem := resolverItem("it.conf", plan.Update)
	linkItem.Change.Kind = plan.KindLink
	f := newResolverDeps(plan.Push, linkItem)
	f.sides[resolverDst+"it.conf"] = cfgsync.Side{Exists: true, Kind: plan.KindLink, LinkTarget: "/old/target", LinkBroken: true}
	f.sides[resolverSrc+"it.conf"] = cfgsync.Side{Exists: true, Kind: plan.KindLink, LinkTarget: "/new/target"}
	d := resolverRoot(t, f)
	d.mustLoad(t)

	d.press(keyPress('v'))
	d.mustShow(t, "destination")

	view := d.view()
	for _, want := range []string{"link -> /old/target (broken", "link -> /new/target (resolves)"} {
		if !strings.Contains(view, want) {
			t.Errorf("link view lacks %q:\n%s", want, view)
		}
	}
}

func TestSyncDiffViewOfDirectoriesListsDifferingChildren(t *testing.T) {
	dirItem := resolverItem("d", plan.Update)
	dirItem.Change.Kind = plan.KindDir
	file := func(name string, size int64) cfgsync.Child {
		return cfgsync.Child{Name: name, Kind: plan.KindFile, Size: size}
	}
	dst := []cfgsync.Child{file("only-dst", 1), file("changed", 1), file("same", 1)}
	src := []cfgsync.Child{file("changed", 2), file("only-src", 1), file("same", 1)}
	f := newResolverDeps(plan.Push, dirItem)
	f.sides[resolverDst+"d"] = cfgsync.Side{Exists: true, Kind: plan.KindDir, Children: dst}
	f.sides[resolverSrc+"d"] = cfgsync.Side{Exists: true, Kind: plan.KindDir, Children: src}
	d := resolverRoot(t, f)
	d.mustLoad(t)

	d.press(keyPress('v'))
	d.mustShow(t, "destination")

	view := d.view()
	for _, want := range []string{"- only-dst", "~ changed", "+ only-src"} {
		if !strings.Contains(view, want) {
			t.Errorf("directory view lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "same") {
		t.Errorf("directory view lists an identical child:\n%s", view)
	}
}

func TestDirDiffLinesCapsAndCountsTheRest(t *testing.T) {
	var src []cfgsync.Child
	for i := range maxDirChildLines + 5 {
		src = append(src, cfgsync.Child{Name: fmt.Sprintf("n%03d", i), Kind: plan.KindFile})
	}

	lines := dirDiffLines(kit.ForVariant("nadir"), 80, cfgsync.Side{Exists: true, Kind: plan.KindDir}, cfgsync.Side{Exists: true, Kind: plan.KindDir, Children: src})

	if len(lines) != maxDirChildLines+1 || !strings.Contains(lines[len(lines)-1], "and 5 more") {
		t.Errorf("got %d lines ending %q, want %d with a count of the 5 hidden", len(lines), lines[len(lines)-1], maxDirChildLines+1)
	}
}
