package cfgsync

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

var (
	hypr  = resolved(manifest.RootConfig, "hypr")
	kitty = resolved(manifest.RootConfig, "kitty")
)

func twoEntryPlan(t *testing.T, dir plan.Direction) SyncPlan {
	t.Helper()
	return planOf(t, dir, []manifest.ResolvedEntry{hypr, kitty}, map[string][]plan.Change{
		"hypr": {
			change("a.conf", plan.Update, plan.KindFile),
			change("b.conf", plan.Create, plan.KindFile),
			change("old", plan.Delete, plan.KindDir),
			change("old/f.txt", plan.Delete, plan.KindFile),
			change("old/sub", plan.Delete, plan.KindDir),
			change("old/sub/g.txt", plan.Delete, plan.KindFile),
		},
		"kitty": {change("kitty.conf", plan.Create, plan.KindFile)},
	})
}

func runApply(t *testing.T, plan SyncPlan, sel Selection, opts ApplyOptions, r *fakeRunner) ApplyResult {
	t.Helper()
	return newService(t, &fakeInspector{state: state.State{RepoPath: testRepo}}, r).Apply(context.Background(), plan, sel, opts, nil)
}

func TestApply_BuildsOrderedSteps(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{Backup: true}, r)

	var kinds []string
	for _, s := range r.steps {
		switch s.(type) {
		case plan.BackupStep:
			kinds = append(kinds, "backup")
		case plan.CopyStep:
			kinds = append(kinds, "copy")
		case plan.DeleteStep:
			kinds = append(kinds, "delete")
		case plan.SaveStateStep:
			kinds = append(kinds, "state")
		case plan.AppendLogStep:
			kinds = append(kinds, "log")
		}
	}
	want := []string{"backup", "copy", "copy", "delete", "delete", "delete", "delete", "state", "log"}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("step order = %v, want %v", kinds, want)
	}

	copies := stepsOfType[plan.CopyStep](r.steps)
	if copies[0].Src != hypr.RepoPath || copies[0].Dst != hypr.DestPath || copies[0].Direction != plan.Push ||
		!reflect.DeepEqual(copies[0].Paths, []string{"a.conf", "b.conf"}) {
		t.Errorf("hypr copy = %+v", copies[0])
	}
	if !reflect.DeepEqual(copies[1].Paths, []string{"kitty.conf"}) {
		t.Errorf("kitty copy paths = %v", copies[1].Paths)
	}
}

func TestApply_CopyStepsOfDirectoryEntriesAreMarkedDir(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{}, r)

	copies := stepsOfType[plan.CopyStep](r.steps)
	if len(copies) != 2 || !copies[0].Dir || !copies[1].Dir {
		t.Fatalf("copy steps = %+v, want Dir=true on both", copies)
	}
}

func TestApply_FailedStepWithoutErrOrDetailStillReportsNonBlankError(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{result: func(s plan.Step) plan.StepResult {
		if _, ok := s.(plan.CopyStep); ok {
			return plan.StepResult{Status: plan.StepFailed}
		}
		return plan.StepResult{Status: plan.StepOK}
	}}

	res := runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{}, r)

	if res.Err == nil || res.Err.Error() != "step failed" {
		t.Fatalf("Err = %v, want \"step failed\"", res.Err)
	}
}

func TestApply_DeletesRunFilesFirstThenDirsDeepestFirst(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{}, r)

	var got []string
	for _, d := range stepsOfType[plan.DeleteStep](r.steps) {
		got = append(got, d.Path)
		if d.Root != hypr.DestPath {
			t.Errorf("delete root = %s, want %s", d.Root, hypr.DestPath)
		}
	}
	base := hypr.DestPath + "/"
	want := []string{base + "old/f.txt", base + "old/sub/g.txt", base + "old/sub", base + "old"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delete order = %v, want %v", got, want)
	}
}

func TestApply_EmptySelectionRunsNothingAndLogsNothing(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{}
	res := runApply(t, syncPlan, Selection{}, ApplyOptions{Backup: true}, r)
	if !res.Nothing || len(r.steps) != 0 {
		t.Fatalf("Nothing = %v, steps = %d; want true, 0", res.Nothing, len(r.steps))
	}
}

func TestApply_BlockedItemsAreNeverApplied(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	sel := Selection{}
	for i := range syncPlan.Items {
		syncPlan.Items[i].Blocked = true
		sel[syncPlan.Items[i].Key()] = true
	}
	r := &fakeRunner{}
	if res := runApply(t, syncPlan, sel, ApplyOptions{}, r); !res.Nothing || len(r.steps) != 0 {
		t.Fatalf("Nothing = %v, steps = %d; want true, 0", res.Nothing, len(r.steps))
	}
}

func TestNeedsBackup(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	byPath := func(paths ...string) Selection {
		sel := Selection{}
		for _, it := range syncPlan.Items {
			for _, p := range paths {
				if it.Change.Path == p {
					sel[it.Key()] = true
				}
			}
		}
		return sel
	}
	tests := []struct {
		name string
		sel  Selection
		want bool
	}{
		{"only creates", byPath("b.conf", "kitty.conf"), false},
		{"an update", byPath("b.conf", "a.conf"), true},
		{"a delete", byPath("old/f.txt"), true},
		{"nothing", Selection{}, false},
	}
	svc := newService(t, &fakeInspector{}, &fakeRunner{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.NeedsBackup(syncPlan, tt.sel); got != tt.want {
				t.Fatalf("NeedsBackup = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApply_BackupStepCoversOnlyOverwrittenAndDeletedFiles(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{Backup: true}, r)

	backups := stepsOfType[plan.BackupStep](r.steps)
	if len(backups) != 1 {
		t.Fatalf("backup steps = %d, want 1", len(backups))
	}
	got := map[plan.BackupFile]bool{}
	for _, f := range backups[0].Files {
		got[f] = true
	}
	for _, p := range []string{"hypr/a.conf", "hypr/old/f.txt", "hypr/old/sub/g.txt"} {
		if !got[plan.BackupFile{Root: manifest.RootConfig, Path: p}] {
			t.Errorf("backup misses %s", p)
		}
	}
	for _, p := range []string{"hypr/b.conf", "kitty/kitty.conf"} {
		if got[plan.BackupFile{Root: manifest.RootConfig, Path: p}] {
			t.Errorf("backup includes created file %s", p)
		}
	}
	if backups[0].Roots[manifest.RootConfig] != rootDirs[manifest.RootConfig] || backups[0].Direction != plan.Push {
		t.Errorf("backup roots/direction = %v / %v", backups[0].Roots, backups[0].Direction)
	}
}

func TestApply_DeclinedOrUnneededBackupAddsNoBackupStep(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	createOnly := Selection{}
	for _, it := range syncPlan.Items {
		if it.Change.Type == plan.Create {
			createOnly[it.Key()] = true
		}
	}
	tests := []struct {
		name string
		sel  Selection
		opts ApplyOptions
	}{
		{"declined with update selected", selectAll(syncPlan), ApplyOptions{Backup: false}},
		{"accepted but only creates", createOnly, ApplyOptions{Backup: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			runApply(t, syncPlan, tt.sel, tt.opts, r)
			if n := len(stepsOfType[plan.BackupStep](r.steps)); n != 0 {
				t.Fatalf("backup steps = %d, want 0", n)
			}
		})
	}
}

func TestApply_BackupFailureAbortsBeforeAnyWrite(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	boom := errors.New("disk full")
	r := &fakeRunner{result: func(s plan.Step) plan.StepResult {
		if _, ok := s.(plan.BackupStep); ok {
			return plan.StepResult{Status: plan.StepFailed, Err: boom}
		}
		return plan.StepResult{Status: plan.StepOK}
	}}
	res := runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{Backup: true}, r)

	if !errors.Is(res.Err, boom) {
		t.Errorf("Err = %v, want %v", res.Err, boom)
	}
	if len(stepsOfType[plan.CopyStep](r.steps))+len(stepsOfType[plan.DeleteStep](r.steps)) != 0 {
		t.Errorf("writes ran after backup failure: %v", r.steps)
	}
}

func TestApply_FirstFailureStopsKeepsPriorItemsAppliedAndLogsFailed(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	boom := errors.New("permission denied")
	r := &fakeRunner{result: func(s plan.Step) plan.StepResult {
		if c, ok := s.(plan.CopyStep); ok && c.Src == kitty.RepoPath {
			return plan.StepResult{Status: plan.StepFailed, Err: boom}
		}
		return plan.StepResult{Status: plan.StepOK}
	}}
	// Order: hypr copy ok, kitty copy fails, so deletes must not run.
	res := runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{}, r)

	if !errors.Is(res.Err, boom) {
		t.Errorf("Err = %v, want %v", res.Err, boom)
	}
	if n := len(stepsOfType[plan.DeleteStep](r.steps)); n != 0 {
		t.Errorf("delete steps after failure = %d, want 0", n)
	}
	results := map[string]string{}
	for _, it := range res.Items {
		results[it.Key.Path] = it.Result
	}
	if results["hypr/a.conf"] != ItemApplied || results["kitty/kitty.conf"] != ItemNotApplied || results["hypr/old/f.txt"] != ItemNotApplied {
		t.Errorf("item results = %v", results)
	}
	logs := stepsOfType[plan.AppendLogStep](r.steps)
	if len(logs) != 1 || logs[0].Entry.Result != "failed" || logs[0].Entry.Error == "" {
		t.Fatalf("log steps = %+v, want one failed entry with error", logs)
	}
}

func TestApply_PartialCopyReportsOnlyTransferredItemsAsApplied(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{result: func(s plan.Step) plan.StepResult {
		if c, ok := s.(plan.CopyStep); ok && c.Src == hypr.RepoPath {
			return plan.StepResult{Status: plan.StepFailed, Err: errors.New("denied"), Applied: []string{"a.conf"}}
		}
		return plan.StepResult{Status: plan.StepOK}
	}}
	res := runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{}, r)

	results := map[string]string{}
	for _, it := range res.Items {
		results[it.Key.Path] = it.Result
	}
	if results["hypr/a.conf"] != ItemApplied || results["hypr/b.conf"] != ItemNotApplied {
		t.Fatalf("item results = %v", results)
	}
}

func TestApply_SuccessSavesLastSyncAndLogsEveryItem(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Pull)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{}, r)

	states := stepsOfType[plan.SaveStateStep](r.steps)
	if len(states) != 1 || states[0].State.LastSync == nil || states[0].State.LastSync.Direction != "pull" ||
		!states[0].State.LastSync.Time.Equal(testNow) || states[0].State.RepoPath != testRepo {
		t.Errorf("state steps = %+v", states)
	}
	logs := stepsOfType[plan.AppendLogStep](r.steps)
	if len(logs) != 1 {
		t.Fatalf("log steps = %d, want 1", len(logs))
	}
	e := logs[0].Entry
	if e.Op != "sync" || e.Direction != "pull" || e.DryRun || e.Result != "ok" || len(e.Items) != len(syncPlan.Items) {
		t.Errorf("log entry = %+v", e)
	}
}

func TestApply_ForwardsRunnerProgress(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	var ticks int
	newService(t, &fakeInspector{}, &fakeRunner{}).Apply(context.Background(), syncPlan, selectAll(syncPlan), ApplyOptions{}, func(plan.StepProgress) { ticks++ })
	if ticks == 0 {
		t.Fatal("no progress forwarded")
	}
}

func TestApply_DryRunEmitsSameStepsAsRealRun(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	sel := selectAll(syncPlan)
	real, dry := &fakeRunner{}, &fakeRunner{result: func(plan.Step) plan.StepResult {
		return plan.StepResult{Status: plan.StepWouldApply, Detail: "would ..."}
	}}
	runApply(t, syncPlan, sel, ApplyOptions{Backup: true}, real)
	res := runApply(t, syncPlan, sel, ApplyOptions{Backup: true, DryRun: true}, dry)

	if res.Err != nil || len(real.steps) != len(dry.steps) {
		t.Fatalf("err = %v, steps real %d vs dry %d", res.Err, len(real.steps), len(dry.steps))
	}
	for i, s := range real.steps {
		if _, isLog := s.(plan.AppendLogStep); isLog {
			if got := dry.steps[i].(plan.AppendLogStep); !got.Entry.DryRun {
				t.Errorf("dry-run log entry not flagged dry_run: %+v", got.Entry)
			}
			continue
		}
		if !reflect.DeepEqual(s, dry.steps[i]) {
			t.Errorf("step %d differs: real %+v, dry %+v", i, s, dry.steps[i])
		}
	}
	for _, o := range res.Outcomes {
		if o.Result.Status != plan.StepWouldApply {
			t.Errorf("outcome %T status = %v, want WouldApply", o.Step, o.Result.Status)
		}
	}
}
