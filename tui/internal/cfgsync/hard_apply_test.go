package cfgsync

import (
	"errors"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

var hardOpts = ApplyOptions{Backup: true, Mandatory: true, Hard: true}

func TestApply_MandatoryBackupIsTakenEvenWhenNotOffered(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{Hard: true, Mandatory: true}, r)

	if n := len(stepsOfType[plan.BackupStep](r.steps)); n != 1 {
		t.Fatalf("backup steps = %d, want 1 without an offer", n)
	}
	if _, ok := r.steps[0].(plan.BackupStep); !ok {
		t.Fatalf("first step = %T, want BackupStep", r.steps[0])
	}
}

func TestApply_HardBackupFailureRunsNoWriteOrDeleteInEitherDirection(t *testing.T) {
	for _, dir := range []plan.Direction{plan.Push, plan.Pull} {
		syncPlan := twoEntryPlan(t, dir)
		r := &fakeRunner{result: func(s plan.Step) plan.StepResult {
			if _, ok := s.(plan.BackupStep); ok {
				return plan.StepResult{Status: plan.StepFailed, Err: errors.New("disk full")}
			}
			return plan.StepResult{Status: plan.StepOK}
		}}
		res := runApply(t, syncPlan, selectAll(syncPlan), hardOpts, r)

		if res.Err == nil {
			t.Errorf("direction %v: Err is nil, want the backup error", dir)
		}
		writes := len(stepsOfType[plan.CopyStep](r.steps)) + len(stepsOfType[plan.DeleteStep](r.steps)) + len(stepsOfType[plan.WriteFileStep](r.steps))
		if writes != 0 {
			t.Errorf("direction %v: %d write or delete steps ran after backup failure: %v", dir, writes, r.steps)
		}
	}
}

func TestApply_HardPullBackupCoversTheRepoFilesItOverwritesOrDeletes(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Pull)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), hardOpts, r)

	backups := stepsOfType[plan.BackupStep](r.steps)
	if len(backups) != 1 {
		t.Fatalf("backup steps = %d, want 1", len(backups))
	}
	if backups[0].Direction != plan.Pull || backups[0].Roots[manifest.RootConfig] != testRepo+"/config" {
		t.Errorf("backup direction/roots = %v / %v, want pull over the repo tree", backups[0].Direction, backups[0].Roots)
	}
	got := map[plan.BackupFile]bool{}
	for _, f := range backups[0].Files {
		got[f] = true
	}
	for _, p := range []string{"hypr/a.conf", "hypr/old/f.txt", "hypr/old/sub/g.txt"} {
		if !got[plan.BackupFile{Root: manifest.RootConfig, Path: p}] {
			t.Errorf("backup misses repo file %s", p)
		}
	}
	if got[plan.BackupFile{Root: manifest.RootConfig, Path: "hypr/b.conf"}] {
		t.Errorf("backup includes created file hypr/b.conf")
	}
}

func TestApply_HardDeleteOfDirKeptNonEmptyIsNotReportedApplied(t *testing.T) {
	syncPlan := planOf(t, plan.Push, []manifest.ResolvedEntry{hypr}, map[string][]plan.Change{
		"hypr": {change("old", plan.Delete, plan.KindDir)},
	})
	r := &fakeRunner{result: func(s plan.Step) plan.StepResult {
		if _, ok := s.(plan.DeleteStep); ok {
			return plan.StepResult{Status: plan.StepOK, Detail: "kept (not empty)"}
		}
		return plan.StepResult{Status: plan.StepOK}
	}}

	res := runApply(t, syncPlan, selectAll(syncPlan), hardOpts, r)

	if len(res.Items) != 1 || res.Items[0].Result == ItemApplied {
		t.Fatalf("items = %+v, want the kept directory not reported as applied", res.Items)
	}
}

func TestApply_HardPullDeletesConfinedToTheRepoEntryRoot(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Pull)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), hardOpts, r)

	deletes := stepsOfType[plan.DeleteStep](r.steps)
	if len(deletes) != 4 {
		t.Fatalf("delete steps = %d, want 4", len(deletes))
	}
	for _, d := range deletes {
		if d.Root != testRepo+"/config/hypr" {
			t.Errorf("delete %s root = %s, want the repo entry root", d.Path, d.Root)
		}
	}
}
