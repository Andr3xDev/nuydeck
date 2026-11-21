package backup

import (
	"context"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func testBackup() Backup {
	return Backup{
		Dir:       "/home/u/.nuydeck-backups/20261103-100000",
		Direction: plan.Push,
		Roots:     map[manifest.Root]string{manifest.RootConfig: "/home/u/.config"},
		Files:     []string{"config/hypr/a.conf"},
	}
}

func TestPlanRestore_DiffsBackupIntoRecordedRootsAndNeverDeletes(t *testing.T) {
	b := testBackup()
	in := &fakeInspector{diff: diffReturning(
		change("hypr/a.conf", plan.Update, plan.KindFile),
		change("hypr/b.conf", plan.Create, plan.KindFile),
		change("hypr/extra.conf", plan.Delete, plan.KindFile),
	)}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanRestore(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if syncPlan.Direction != plan.Restore {
		t.Errorf("direction = %v, want Restore", syncPlan.Direction)
	}
	if len(in.diffCalls) != 1 || in.diffCalls[0] != (diffCall{b.Dir + "/config", "/home/u/.config"}) {
		t.Errorf("diff calls = %+v", in.diffCalls)
	}
	for _, it := range syncPlan.Items {
		if it.Change.Type == plan.Delete {
			t.Errorf("restore offered deletion of %s", it.Change.Path)
		}
	}
	if len(syncPlan.Items) != 2 {
		t.Errorf("items = %d, want 2", len(syncPlan.Items))
	}
}

func TestApply_RestoreCopiesFromBackupAndOffersBackupForOverwrite(t *testing.T) {
	b := testBackup()
	in := &fakeInspector{diff: diffReturning(change("hypr/a.conf", plan.Update, plan.KindFile))}
	svc := newService(t, in, &fakeRunner{})
	syncPlan, err := svc.PlanRestore(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	sel := selectAll(syncPlan)
	if !svc.NeedsBackup(syncPlan, sel) {
		t.Error("restore overwriting a file must offer a backup")
	}
	r := &fakeRunner{}
	newService(t, in, r).Apply(context.Background(), syncPlan, sel, cfgsync.ApplyOptions{Backup: true}, nil)

	copies := stepsOfType[plan.CopyStep](r.steps)
	if len(copies) != 1 || copies[0].Direction != plan.Restore || copies[0].Src != b.Dir+"/config" ||
		copies[0].Dst != "/home/u/.config" || len(copies[0].Paths) != 1 || copies[0].Paths[0] != "hypr/a.conf" {
		t.Fatalf("copy steps = %+v", copies)
	}
	if len(stepsOfType[plan.DeleteStep](r.steps)) != 0 {
		t.Error("restore must not delete")
	}
}
