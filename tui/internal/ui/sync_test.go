package ui

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type fakeSyncDeps struct {
	plan          cfgsync.SyncPlan
	needsBackup   bool
	progressPaths []string
	blockUntilCtx bool
	result        cfgsync.ApplyResult
	opts          cfgsync.ApplyOptions
	applyCalls    atomic.Int32
}

func (f *fakeSyncDeps) deps() Deps {
	return Deps{PlanSync: f.PlanSync, NeedsBackup: f.NeedsBackup, ApplySync: f.Apply}
}

func (f *fakeSyncDeps) PlanSync(context.Context, plan.Direction, []manifest.ResolvedEntry) (cfgsync.SyncPlan, error) {
	return f.plan, nil
}

func (f *fakeSyncDeps) NeedsBackup(cfgsync.SyncPlan, cfgsync.Selection) bool { return f.needsBackup }

func (f *fakeSyncDeps) Apply(ctx context.Context, _ cfgsync.SyncPlan, _ cfgsync.Selection, opts cfgsync.ApplyOptions, progress func(plan.StepProgress)) cfgsync.ApplyResult {
	f.applyCalls.Add(1)
	f.opts = opts
	for _, p := range f.progressPaths {
		progress(plan.StepProgress{Path: p})
	}
	if f.blockUntilCtx {
		<-ctx.Done()
	}
	return f.result
}

func selectedPlan() cfgsync.SyncPlan {
	item := planItem("a", false)
	return cfgsync.SyncPlan{Items: []cfgsync.PlanItem{item}, Selection: cfgsync.Selection{item.Key(): true}}
}

// runCmds feeds every command result back into the screen until no command is left.
func runCmds(s *Sync, cmd tea.Cmd) {
	for cmd != nil {
		_, cmd = s.Update(cmd())
	}
}

func stagedOf(s *Sync) *syncStaged { return s.flow.st.(*syncStaged) }

func planItem(path string, blocked bool) cfgsync.PlanItem {
	return cfgsync.PlanItem{Change: plan.Change{Entry: manifest.Entry{Root: "home", Path: path}, Type: plan.Update}, Blocked: blocked}
}

func newPreview(f *fakeSyncDeps, dryRun bool) *Sync {
	svc := testServices(f.deps())
	svc.dryRun = dryRun
	s := newSync(svc)
	runCmds(s, s.startPlan(plan.Push, nil))
	return s
}

// confirmedRequest accepts the preview and the confirm (the plan must need no backup) and returns the run the flow asks for.
func confirmedRequest(t *testing.T, s *Sync) runRequest {
	t.Helper()
	s.Update(keyPress(tea.KeyEnter))
	_, cmd := s.Update(keyPress('y'))
	if !isRunRequest(cmd) {
		t.Fatalf("stage = %v, want the confirm to return a run request", s.flow.stage)
	}
	return cmd().(runRequest)
}

// runWork runs the request's work to its end and returns the lines it emitted.
func runWork(req runRequest) (lines []string, end runEnd) {
	end = req.work(context.Background(), func(e plan.Event) { lines = append(lines, e.Line) })
	return lines, end
}

func keyPress(code rune) tea.KeyPressMsg {
	if code == tea.KeySpace {
		return tea.KeyPressMsg{Code: code, Text: " "}
	}
	if code == tea.KeyEnter {
		return tea.KeyPressMsg{Code: code}
	}
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

func TestSyncPreviewExposesPlanNoticesToTheBand(t *testing.T) {
	const notice = "skipped: source missing: home/.gone"
	plan := selectedPlan()
	plan.Notices = []string{notice}
	s := newPreview(&fakeSyncDeps{plan: plan}, false)

	if got := s.Notices(); len(got) != 1 || got[0] != notice {
		t.Fatalf("Notices() = %q, want [%q]", got, notice)
	}
	if strings.Contains(ansi.Strip(s.View(120, 20)), notice) {
		t.Fatalf("the preview must leave %q to the band", notice)
	}
}

func TestSyncToggleRespectsBlockedItems(t *testing.T) {
	plan := cfgsync.SyncPlan{Items: []cfgsync.PlanItem{planItem("a", false), planItem("b", true)}, Selection: cfgsync.Selection{}}
	plan.Selection[plan.Items[0].Key()] = true
	s := newPreview(&fakeSyncDeps{plan: plan}, false)
	st := stagedOf(s)

	s.Update(keyPress(tea.KeySpace))
	if st.sel[plan.Items[0].Key()] {
		t.Error("space must deselect the selected item")
	}
	s.Update(keyPress('j'))
	if st.cursor != 1 {
		t.Fatalf("cursor = %d, want j to move the item cursor", st.cursor)
	}
	s.Update(keyPress(tea.KeySpace))
	if st.sel[plan.Items[1].Key()] {
		t.Error("blocked item must not be selectable")
	}
	s.Update(keyPress('a'))
	if !st.sel[plan.Items[0].Key()] || st.sel[plan.Items[1].Key()] || st.selectedCount() != 1 {
		t.Errorf("a must select only selectable items: %v", st.sel)
	}
	s.Update(keyPress('a'))
	if st.selectedCount() != 0 {
		t.Error("a on a full selection must clear it")
	}
}

func TestSyncEnterDialogsDeclineBackup(t *testing.T) {
	item := planItem("a", false)
	plan := cfgsync.SyncPlan{Items: []cfgsync.PlanItem{item}, Selection: cfgsync.Selection{item.Key(): true}}
	s := newPreview(&fakeSyncDeps{plan: plan, needsBackup: true}, false)

	s.Update(keyPress(tea.KeyEnter))
	if s.flow.stage != stageOffer || !claims(s.Keys(), "esc") {
		t.Fatalf("stage = %v, want the backup offer", s.flow.stage)
	}
	if prompt, _ := stagedOf(s).Offer(); prompt != backupOfferText {
		t.Errorf("offer = %q, want %q", prompt, backupOfferText)
	}
	s.Update(keyPress('n'))
	if s.flow.stage != stageConfirm {
		t.Fatalf("stage = %v, want the confirm after declining the backup", s.flow.stage)
	}
	s.Update(specialKey(tea.KeyEscape))
	s.Update(specialKey(tea.KeyEscape))
	if s.flow.stage != stagePreview {
		t.Errorf("stage = %v, want esc to step back through the offer to the preview", s.flow.stage)
	}
}

func TestSyncOffersNoBackupWhenNothingIsOverwritten(t *testing.T) {
	s := newPreview(&fakeSyncDeps{plan: selectedPlan()}, false)

	s.Update(keyPress(tea.KeyEnter))
	if s.flow.stage != stageConfirm {
		t.Errorf("stage = %v, want the confirm without an offer", s.flow.stage)
	}
}

func TestSyncDiscardsStalePlan(t *testing.T) {
	s := newPreview(&fakeSyncDeps{}, false)
	stale := s.startPlan(plan.Push, nil)()
	s.startPlan(plan.Pull, nil)
	s.Update(stale)
	if s.flow.stage != stagePlanning {
		t.Error("stale plan must be discarded")
	}
}

func TestSyncApplyLoopForwardsOptions(t *testing.T) {
	tests := []struct {
		name       string
		dryRun     bool
		backupKey  rune
		wantBackup bool
	}{
		{"backup accepted in dry-run", true, 'y', true},
		{"backup declined on real run", false, 'n', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeSyncDeps{plan: selectedPlan(), needsBackup: true, progressPaths: []string{"a", "b"}}
			s := newPreview(f, tt.dryRun)

			s.Update(keyPress(tea.KeyEnter))
			s.Update(keyPress(tt.backupKey))
			_, cmd := s.Update(keyPress('y'))
			if !isRunRequest(cmd) {
				t.Fatalf("stage = %v, want a run request", s.flow.stage)
			}
			lines, end := runWork(cmd().(runRequest))

			want := cfgsync.ApplyOptions{Backup: tt.wantBackup, DryRun: tt.dryRun}
			if _, broadcast := end.msg.(applyDoneMsg); f.opts != want || len(lines) != 2 || !broadcast {
				t.Errorf("opts=%+v lines=%v end=%+v", f.opts, lines, end)
			}
		})
	}
}

func TestSyncNothingSelectedDoesNotApply(t *testing.T) {
	f := &fakeSyncDeps{plan: cfgsync.SyncPlan{Items: []cfgsync.PlanItem{planItem("a", false)}}}
	s := newPreview(f, false)

	_, cmd := s.Update(keyPress(tea.KeyEnter))
	if cmd != nil || s.flow.stage != stagePreview || claims(s.Keys(), "enter") || f.applyCalls.Load() != 0 {
		t.Errorf("cmd=%v stage=%v applyCalls=%d, want the preview with no apply key", cmd != nil, s.flow.stage, f.applyCalls.Load())
	}
}

// syncRunningRoot drives a Root through push, preview, confirm and run, and returns once the apply is blocked in the run.
func syncRunningRoot(t *testing.T, f *fakeSyncDeps) *driver {
	t.Helper()
	f.blockUntilCtx = true
	root := NewRoot("", false, f.deps())
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	d := newDriver(t, root)
	_, cmd := root.Update(startSyncMsg{direction: plan.Push})
	d.launch(cmd)
	s, _, _ := sectionOf[*Sync](root)
	d.must(t, "the preview", func() bool { return s.flow.stage == stagePreview })
	d.press(keyEnter)
	d.press(keyPress('y'))
	d.must(t, "the apply to start", func() bool { return root.run != nil && f.applyCalls.Load() == 1 })
	return d
}

func TestRootIgnoresNewPlansWhileApplying(t *testing.T) {
	d := syncRunningRoot(t, &fakeSyncDeps{plan: selectedPlan()})
	s, _, _ := sectionOf[*Sync](d.root)
	before, run := s.flow, d.root.run

	_, cmd := d.root.Update(startSyncMsg{direction: plan.Pull})

	if cmd != nil || s.flow != before || d.root.run != run || d.runDone() {
		t.Errorf("a new plan changed the running apply: cmd=%v sameFlow=%v sameRun=%v done=%v", cmd != nil, s.flow == before, d.root.run == run, d.runDone())
	}
}

func TestRootQuitCancelsRunningApplyWithoutHanging(t *testing.T) {
	d := syncRunningRoot(t, &fakeSyncDeps{plan: selectedPlan(), progressPaths: []string{"a"}})

	d.press(keyCtrlC)
	d.must(t, "the cancelled apply to end", d.runDone)
	d.press(keyPress('q'))
	d.must(t, "the program to quit", func() bool { return d.quit })

	begin := time.Now()
	d.root.WaitInstall(shutdownTimeout)
	if elapsed := time.Since(begin); elapsed > promptReturn {
		t.Errorf("WaitInstall took %v, want the apply to be over already", elapsed)
	}
}

func TestSyncApplyReportsItsResultAsRunLines(t *testing.T) {
	f := &fakeSyncDeps{plan: selectedPlan(), result: cfgsync.ApplyResult{
		Outcomes: []cfgsync.StepOutcome{{Result: plan.StepResult{Status: plan.StepWouldApply, Detail: "would append log entry"}}},
		Items:    []cfgsync.ItemResult{{Key: cfgsync.ItemKey{Root: "home", Path: "a"}, Result: cfgsync.ItemNotApplied}},
		Err:      errors.New("boom"),
	}}
	lines, end := runWork(confirmedRequest(t, newPreview(f, true)))

	if got := strings.Join(lines, "\n"); !strings.Contains(got, "would append log entry") || !strings.Contains(got, "not applied: home:a") {
		t.Errorf("run lines = %q, want the dry-run detail and the item result", got)
	}
	if end.err == nil || end.err.Error() != "boom" {
		t.Errorf("end error = %v, want boom", end.err)
	}

	lines, _ = runWork(confirmedRequest(t, newPreview(&fakeSyncDeps{plan: selectedPlan(), result: cfgsync.ApplyResult{Nothing: true}}, false)))
	if len(lines) != 1 || lines[0] != nothingDoneText {
		t.Errorf("run lines = %q, want the nothing-done line", lines)
	}
}

func TestResultLines_KeptDirNamesItsSanitizedPathAndNoticesAreAppended(t *testing.T) {
	res := cfgsync.ApplyResult{
		Outcomes: []cfgsync.StepOutcome{{
			Step:   plan.DeleteStep{Path: "/home/u/.config/hy\x1bpr"},
			Result: plan.StepResult{Status: plan.StepOK, Detail: plan.KeptNotEmpty},
		}},
		Notices: []string{"last sync not recorded: state.toml could not be read"},
	}
	got := resultLines(res)
	want := []string{"/home/u/.config/hypr: " + plan.KeptNotEmpty, res.Notices[0]}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("resultLines = %q, want %q", got, want)
	}
}
