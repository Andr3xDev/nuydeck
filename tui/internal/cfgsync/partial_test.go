package cfgsync

import (
	"context"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// partialService is a service whose manifest is hypr and kitty.
func partialService(t *testing.T, in *fakeInspector, r plan.Runner) *Service {
	t.Helper()
	svc := newService(t, in, r)
	svc.AllEntries = []manifest.ResolvedEntry{hypr, kitty}
	return svc
}

func TestPlanSync_SubsetOfTheManifestIsPartialAndWholeIsNot(t *testing.T) {
	in := &fakeInspector{diff: diffReturning(change("a.conf", plan.Update, plan.KindFile))}
	svc := partialService(t, in, &fakeRunner{})

	part, err := svc.PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{hypr})
	if err != nil {
		t.Fatal(err)
	}
	whole, err := svc.PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{hypr, kitty})
	if err != nil {
		t.Fatal(err)
	}

	if !part.Partial || whole.Partial {
		t.Errorf("Partial: subset=%v whole=%v, want true and false", part.Partial, whole.Partial)
	}
}

func TestPlanSync_LinkIntoANonPlannedManagedEntryIsNotWarned(t *testing.T) {
	in := &fakeInspector{
		diff:        diffReturning(change("in.lnk", plan.Create, plan.KindLink)),
		linkTargets: map[string]string{hypr.RepoPath + "/in.lnk": "../kitty/kitty.conf"},
	}
	svc := partialService(t, in, &fakeRunner{})

	partial, err := svc.PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{hypr})
	if err != nil {
		t.Fatal(err)
	}
	svc.AllEntries = nil
	alone, err := svc.PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{hypr})
	if err != nil {
		t.Fatal(err)
	}

	if got := partial.Items[0].Change.Warning; got != plan.NoLinkWarning {
		t.Errorf("partial plan warning = %v, want none for a link into kitty", got)
	}
	if got := alone.Items[0].Change.Warning; got != plan.WarnLeavesManagedSet {
		t.Errorf("without the whole manifest the warning = %v, want it to leave the planned set", got)
	}
}

func TestApply_PartialPlanLogsButNeverRecordsLastSync(t *testing.T) {
	in := &fakeInspector{
		diff:  diffReturning(change("a.conf", plan.Update, plan.KindFile)),
		state: state.State{RepoPath: testRepo},
	}
	r := &fakeRunner{}
	svc := partialService(t, in, r)
	syncPlan, err := svc.PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{hypr})
	if err != nil {
		t.Fatal(err)
	}

	svc.Apply(context.Background(), syncPlan, selectAll(syncPlan), ApplyOptions{}, nil)

	if saves := stepsOfType[plan.SaveStateStep](r.steps); len(saves) != 0 {
		t.Errorf("a partial apply recorded the last sync: %+v", saves)
	}
	if logs := stepsOfType[plan.AppendLogStep](r.steps); len(logs) != 1 {
		t.Errorf("log steps = %d, want 1", len(logs))
	}
}

func TestPlanHard_ScopedRunOnlyTouchesTheScopedEntry(t *testing.T) {
	in := &fakeInspector{diff: diffReturning(change("a.conf", plan.Update, plan.KindFile), change("extra.conf", plan.Delete, plan.KindFile))}
	r := &fakeRunner{}
	svc := partialService(t, in, r)
	hp, err := PlanHard(context.Background(), svc, plan.Push, []manifest.ResolvedEntry{kitty})
	if err != nil {
		t.Fatal(err)
	}

	svc.Apply(context.Background(), hp.SyncPlan, hp.Selection, hardOpts, nil)

	deletes := stepsOfType[plan.DeleteStep](r.steps)
	if len(deletes) != 1 || !strings.HasPrefix(deletes[0].Path, kitty.DestPath+"/") {
		t.Errorf("deletes = %+v, want only extra.conf under %s", deletes, kitty.DestPath)
	}
	for _, c := range stepsOfType[plan.CopyStep](r.steps) {
		if c.Entry.Entry != kitty.Entry {
			t.Errorf("copy touched %+v, want only kitty", c.Entry.Entry)
		}
	}
	for _, b := range stepsOfType[plan.BackupStep](r.steps) {
		for _, f := range b.Files {
			if strings.Contains(f.Path, "hypr") {
				t.Errorf("backup covers hypr: %+v", f)
			}
		}
	}
}
