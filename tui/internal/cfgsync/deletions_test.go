package cfgsync

import (
	"context"
	"reflect"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

func TestDefaultSelected(t *testing.T) {
	item := func(typ plan.ChangeType, blocked, ignored bool) PlanItem {
		return PlanItem{Change: plan.Change{Path: "a", Type: typ, Kind: plan.KindFile}, Blocked: blocked, Ignored: ignored}
	}
	tests := []struct {
		name string
		item PlanItem
		want bool
	}{
		{"update", item(plan.Update, false, false), true},
		{"create", item(plan.Create, false, false), true},
		{"delete", item(plan.Delete, false, false), false},
		{"blocked", item(plan.Update, true, false), false},
		{"ignored", item(plan.Update, false, true), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultSelected(tt.item); got != tt.want {
				t.Fatalf("defaultSelected = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanSync_DeletionsUnselectedAndIgnoredItemsFlagged(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	in := &fakeInspector{
		diff: diffReturning(
			change("a.conf", plan.Update, plan.KindFile),
			change("b.conf", plan.Update, plan.KindFile),
			change("gone.conf", plan.Delete, plan.KindFile),
		),
		state: state.State{Ignores: []string{"config:hypr/b.conf"}},
	}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	type got struct{ selected, ignored bool }
	have := map[string]got{}
	for _, it := range syncPlan.Items {
		have[it.Change.Path] = got{syncPlan.Selection[it.Key()], it.Ignored}
	}
	want := map[string]got{
		"a.conf":    {true, false},
		"b.conf":    {false, true},
		"gone.conf": {false, false},
	}
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("selected/ignored = %v, want %v", have, want)
	}
}

func TestApply_NormalModeNeverEmitsDeleteStepEvenWhenDeletionIsForcedIntoSelection(t *testing.T) {
	for _, dir := range []plan.Direction{plan.Push, plan.Pull} {
		syncPlan := twoEntryPlan(t, dir)
		r := &fakeRunner{}
		runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{Backup: true}, r)
		if n := len(stepsOfType[plan.DeleteStep](r.steps)); n != 0 {
			t.Errorf("direction %v: %d DeleteStep in normal mode, want 0", dir, n)
		}
	}
}

func TestApply_HardEmitsDeletesFilesFirstThenDirsDeepestFirstConfinedToRoot(t *testing.T) {
	syncPlan := twoEntryPlan(t, plan.Push)
	r := &fakeRunner{}
	runApply(t, syncPlan, selectAll(syncPlan), ApplyOptions{Hard: true, Mandatory: true, Backup: true}, r)

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
