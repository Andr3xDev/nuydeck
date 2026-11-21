package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	modeNormalLabel = "Normal"
	modeHardLabel   = "Hard"
)

// modeDeps fakes the sync deps of the mode popup; it records which planner and which apply options ran.
type modeDeps struct {
	mu         sync.Mutex
	syncDirs   []plan.Direction
	hardDirs   []plan.Direction
	hardItems  []cfgsync.PlanItem
	applied    cfgsync.SyncPlan
	appliedSel cfgsync.Selection
	opts       cfgsync.ApplyOptions
	applyCalls atomic.Int32
}

// hardConfirmFor is the confirm text the fake PlanHard hands over; short lines so the 50-cell dialog never splits them.
func hardConfirmFor(dir plan.Direction) string {
	if dir == plan.Pull {
		return "Hard reinstall: PC -> repo\ndelete home:extra"
	}
	return "Hard reinstall: repo -> PC\ndelete home:extra"
}

func newModeDeps() *modeDeps {
	return &modeDeps{hardItems: []cfgsync.PlanItem{resolverItem("a", plan.Update), resolverItem("extra", plan.Delete)}}
}

func (f *modeDeps) deps() Deps {
	return Deps{PlanSync: f.PlanSync, PlanHard: f.PlanHard, NeedsBackup: f.NeedsBackup, ApplySync: f.Apply}
}

func (f *modeDeps) PlanSync(_ context.Context, dir plan.Direction, _ []manifest.ResolvedEntry) (cfgsync.SyncPlan, error) {
	f.mu.Lock()
	f.syncDirs = append(f.syncDirs, dir)
	f.mu.Unlock()
	item := resolverItem("a", plan.Update)
	return cfgsync.SyncPlan{Direction: dir, Items: []cfgsync.PlanItem{item}, Selection: cfgsync.Selection{item.Key(): true}}, nil
}

func (f *modeDeps) PlanHard(_ context.Context, dir plan.Direction, _ []manifest.ResolvedEntry) (cfgsync.HardPlan, error) {
	f.mu.Lock()
	f.hardDirs = append(f.hardDirs, dir)
	items := f.hardItems
	f.mu.Unlock()
	sel := cfgsync.Selection{}
	for _, it := range items {
		sel[it.Key()] = true
	}
	return cfgsync.HardPlan{
		SyncPlan:  cfgsync.SyncPlan{Direction: dir, Items: items, Selection: sel},
		Selection: sel,
		Confirm:   hardConfirmFor(dir),
	}, nil
}

// NeedsBackup is always true so that a Hard flow showing no offer proves the offer is skipped, not merely unneeded.
func (f *modeDeps) NeedsBackup(cfgsync.SyncPlan, cfgsync.Selection) bool { return true }

func (f *modeDeps) Apply(_ context.Context, p cfgsync.SyncPlan, sel cfgsync.Selection, opts cfgsync.ApplyOptions, _ func(plan.StepProgress)) cfgsync.ApplyResult {
	f.mu.Lock()
	f.applied, f.appliedSel, f.opts = p, sel, opts
	f.mu.Unlock()
	f.applyCalls.Add(1)
	return cfgsync.ApplyResult{}
}

func (f *modeDeps) planned() (syncDirs, hardDirs []plan.Direction) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]plan.Direction(nil), f.syncDirs...), append([]plan.Direction(nil), f.hardDirs...)
}

func (f *modeDeps) applyArgs() (cfgsync.SyncPlan, cfgsync.Selection, cfgsync.ApplyOptions) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.applied, f.appliedSel, f.opts
}

// modeRoot is a Root with the Sync panel focused and idle, so P and p reach the screen as the program delivers them.
func modeRoot(t *testing.T, f *modeDeps) (*driver, *Sync) {
	t.Helper()
	root := NewRoot("", false, f.deps())
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	s, at, _ := sectionOf[*Sync](root)
	root.selected, root.focus = at, focusPanel
	return newDriver(t, root), s
}

func inStage(s *Sync, stage flowStage) func() bool {
	return func() bool { return s.flow.active() && s.flow.stage == stage }
}

// openHardConfirm picks Hard for the direction key and goes on from the read-only list to the confirm.
func openHardConfirm(t *testing.T, d *driver, s *Sync, direction tea.KeyPressMsg) {
	t.Helper()
	d.press(direction)
	d.press(keyDown)
	d.press(keyEnter)
	d.must(t, "the Hard list", inStage(s, stagePreview))
	d.press(keyEnter)
	d.must(t, "the Hard confirm", inStage(s, stageConfirm))
}

var modeDirections = []struct {
	name string
	key  tea.KeyPressMsg
	dir  plan.Direction
}{
	{"P is push", keyPress('P'), plan.Push},
	{"p is pull", keyPress('p'), plan.Pull},
}

func TestSyncModePopupOpensOnPushAndPullWithTheDirectionLabel(t *testing.T) {
	for _, tt := range modeDirections {
		t.Run(tt.name, func(t *testing.T) {
			f := newModeDeps()
			d, s := modeRoot(t, f)
			if strings.Contains(d.view(), modeHardLabel) {
				t.Fatalf("the idle Sync screen already shows %q", modeHardLabel)
			}

			d.press(tt.key)

			view := d.view()
			for _, want := range []string{modeNormalLabel, modeHardLabel, directionLabel(tt.dir)} {
				if !strings.Contains(view, want) {
					t.Errorf("popup view lacks %q:\n%s", want, view)
				}
			}
			if !claims(s.Keys(), "enter") || !claims(s.Keys(), "esc") {
				t.Error("the open popup must claim enter and esc in the keybar")
			}
			d.settle()
			if syncs, hards := f.planned(); len(syncs)+len(hards) != 0 || s.flow.active() {
				t.Errorf("opening the popup planned something: sync=%v hard=%v", syncs, hards)
			}
		})
	}
}

func TestSyncModePopupEscCancelsWithoutPlanning(t *testing.T) {
	for _, tt := range modeDirections {
		t.Run(tt.name, func(t *testing.T) {
			f := newModeDeps()
			d, s := modeRoot(t, f)
			d.press(tt.key)

			d.press(keyEsc)

			d.settle()
			syncs, hards := f.planned()
			if strings.Contains(d.view(), modeHardLabel) || s.flow.active() || len(syncs)+len(hards) != 0 {
				t.Errorf("esc must close the popup and plan nothing: sync=%v hard=%v active=%v", syncs, hards, s.flow.active())
			}
			if d.root.focus != focusPanel {
				t.Error("esc at the popup must not leave the panel")
			}
		})
	}
}

func TestSyncModePopupPicksNormalOrHardForTheChosenDirection(t *testing.T) {
	picks := []struct {
		name string
		keys []tea.KeyPressMsg
		hard bool
	}{
		{"enter takes the preselected Normal", nil, false},
		{"down then enter takes Hard", []tea.KeyPressMsg{keyDown}, true},
		{"down then up then enter is Normal again", []tea.KeyPressMsg{keyDown, keyUp}, false},
	}
	for _, dir := range modeDirections {
		for _, pick := range picks {
			t.Run(dir.name+"/"+pick.name, func(t *testing.T) {
				f := newModeDeps()
				d, s := modeRoot(t, f)
				d.press(dir.key)
				for _, k := range pick.keys {
					d.press(k)
				}

				d.press(keyEnter)

				d.must(t, "the preview", inStage(s, stagePreview))
				syncs, hards := f.planned()
				wantSync, wantHard := []plan.Direction{dir.dir}, []plan.Direction(nil)
				if pick.hard {
					wantSync, wantHard = wantHard, wantSync
				}
				if !slices.Equal(syncs, wantSync) || !slices.Equal(hards, wantHard) {
					t.Errorf("planned sync=%v hard=%v, want sync=%v hard=%v", syncs, hards, wantSync, wantHard)
				}
				if s.flow.asked {
					t.Error("a Hard flow must not show the backup offer")
				}
			})
		}
	}
}

func TestSyncHardConfirmShowsTheHardConfirmAndYAppliesForcedHard(t *testing.T) {
	for _, tt := range modeDirections {
		t.Run(tt.name, func(t *testing.T) {
			f := newModeDeps()
			d, s := modeRoot(t, f)
			openHardConfirm(t, d, s, tt.key)

			for _, line := range []string{firstLine(hardConfirmFor(tt.dir)), "delete home:extra"} {
				if !strings.Contains(d.view(), line) {
					t.Errorf("confirm view lacks %q:\n%s", line, d.view())
				}
			}
			if f.applyCalls.Load() != 0 {
				t.Fatal("the confirm must not apply anything before y")
			}

			d.press(keyPress('y'))
			d.must(t, "the apply to start", func() bool { return f.applyCalls.Load() == 1 })

			p, sel, opts := f.applyArgs()
			if want := (cfgsync.ApplyOptions{Backup: true, Mandatory: true, Hard: true}); opts != want {
				t.Errorf("apply options = %+v, want %+v", opts, want)
			}
			if p.Direction != tt.dir {
				t.Errorf("applied direction = %v, want %v", p.Direction, tt.dir)
			}
			if !sel[itemKey("a")] || !sel[itemKey("extra")] {
				t.Errorf("applied selection = %v, want the hard selection with the deletion", sel)
			}
		})
	}
}

func TestSyncHardConfirmAnyKeyButYReturnsWithNothingRun(t *testing.T) {
	declines := map[string]tea.KeyPressMsg{
		"n": keyPress('n'), "x": keyPress('x'), "enter": keyEnter, "space": keyPress(tea.KeySpace),
	}
	for name, k := range declines {
		t.Run(name, func(t *testing.T) {
			f := newModeDeps()
			d, s := modeRoot(t, f)
			openHardConfirm(t, d, s, keyPress('P'))

			d.press(k)

			d.settle()
			if f.applyCalls.Load() != 0 || s.flow.active() || d.root.run != nil {
				t.Errorf("%s: applyCalls=%d flowActive=%v run=%v, want a clean return with no step run", name, f.applyCalls.Load(), s.flow.active(), d.root.run != nil)
			}
		})
	}
}

// Esc at the confirm steps back to the list, like every flow; nothing runs.
func TestSyncHardConfirmEscReturnsToTheListWithNothingRun(t *testing.T) {
	f := newModeDeps()
	d, s := modeRoot(t, f)
	openHardConfirm(t, d, s, keyPress('P'))

	d.press(keyEsc)

	d.must(t, "the Hard list", inStage(s, stagePreview))
	if f.applyCalls.Load() != 0 {
		t.Error("esc applied something")
	}
}

func TestSyncHardConfirmScrollsToTheLastDeletionWithoutDeclining(t *testing.T) {
	const deletions = 60
	f := newModeDeps()
	f.hardItems = nil
	for i := range deletions {
		f.hardItems = append(f.hardItems, resolverItem(fmt.Sprintf("extra%02d", i), plan.Delete))
	}
	d, s := modeRoot(t, f)
	openHardConfirm(t, d, s, keyPress('P'))
	last := fmt.Sprintf("extra%02d", deletions-1)
	f.mu.Lock()
	s.flow.built.Confirm = ""
	for i := range deletions {
		s.flow.built.Confirm += fmt.Sprintf("delete home:extra%02d\n", i)
	}
	f.mu.Unlock()
	if strings.Contains(d.view(), last) || !strings.Contains(d.view(), "scroll") {
		t.Fatalf("the long list must be clipped with a scroll hint:\n%s", d.view())
	}

	for range 2 * deletions {
		d.press(keyDown)
	}

	if !s.flow.active() || s.flow.stage != stageConfirm {
		t.Fatal("scroll keys must not decline the confirm")
	}
	if !strings.Contains(d.view(), last) {
		t.Errorf("scrolling down never reached %q:\n%s", last, d.view())
	}
	if f.applyCalls.Load() != 0 {
		t.Error("scrolling must not apply anything")
	}
}

// A hard plan whose only work is deleting extras still has to reach the confirm.
func TestSyncHardReachesTheConfirmWhenItOnlyDeletes(t *testing.T) {
	f := newModeDeps()
	f.hardItems = []cfgsync.PlanItem{resolverItem("extra", plan.Delete)}
	d, s := modeRoot(t, f)

	openHardConfirm(t, d, s, keyPress('P'))
}

// Spec: ignored items are never overwritten by a hard run, and hard has no resolver to opt them in.
func TestSyncHardNothingApplicableCannotBeToggledIntoAnApply(t *testing.T) {
	f := newModeDeps()
	ignored := resolverItem("ignored", plan.Update)
	ignored.Ignored = true
	hardPlan := cfgsync.HardPlan{
		SyncPlan:  cfgsync.SyncPlan{Direction: plan.Push, Items: []cfgsync.PlanItem{ignored}, Selection: cfgsync.Selection{}},
		Selection: cfgsync.Selection{},
		Confirm:   hardConfirmFor(plan.Push),
	}
	deps := f.deps()
	deps.PlanHard = func(context.Context, plan.Direction, []manifest.ResolvedEntry) (cfgsync.HardPlan, error) {
		return hardPlan, nil
	}
	root := NewRoot("", false, deps)
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	s, at, _ := sectionOf[*Sync](root)
	root.selected, root.focus = at, focusPanel
	d := newDriver(t, root)
	d.press(keyPress('P'))
	d.press(keyDown)
	d.press(keyEnter)
	d.must(t, "the preview", inStage(s, stagePreview))

	d.press(keyPress(tea.KeySpace))
	d.press(keyEnter)
	d.press(keyPress('y'))
	d.settle()

	if s.flow.stage == stageConfirm || f.applyCalls.Load() != 0 {
		t.Errorf("an ignored item reached the hard apply: stage=%v applies=%d", s.flow.stage, f.applyCalls.Load())
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}
