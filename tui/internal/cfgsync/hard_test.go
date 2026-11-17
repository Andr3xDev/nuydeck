package cfgsync

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const notTouchedHeader = "Not touched (ignored or blocked):"

// hardFixture plans a Hard run over one entry with every kind of item Hard must treat differently,
// plus an entry whose source side is missing.
func hardFixture(t *testing.T, dir plan.Direction) (HardPlan, *Service, *fakeRunner) {
	t.Helper()
	hyprGPU := resolved(manifest.RootConfig, "hypr", "configs/gpu_env.lua")
	gone := resolved(manifest.RootConfig, "gone")
	dstOfHypr, srcOfGone := hyprGPU.DestPath, gone.RepoPath
	if dir == plan.Pull {
		dstOfHypr, srcOfGone = hyprGPU.RepoPath, gone.DestPath
	}
	in := &fakeInspector{
		diff: func(e manifest.ResolvedEntry, _, _ plan.Endpoint) ([]plan.Change, error) {
			if e.Path != "hypr" {
				return nil, nil
			}
			return []plan.Change{
				change("a.conf", plan.Update, plan.KindFile),
				change("b.conf", plan.Create, plan.KindFile),
				change("extra.conf", plan.Delete, plan.KindFile),
				change("old", plan.Delete, plan.KindDir),
				change("old/f.txt", plan.Delete, plan.KindFile),
				change("ignored.conf", plan.Update, plan.KindFile),
				change("ignored-extra.conf", plan.Delete, plan.KindFile),
				change("configs/gpu_env.lua", plan.Delete, plan.KindFile),
				change("sub/c.conf", plan.Create, plan.KindFile),
			}, nil
		},
		kinds: map[string]plan.NodeKind{dstOfHypr + "/sub": plan.KindLink, srcOfGone: plan.KindNone},
		state: state.State{Ignores: []string{"config:hypr/ignored.conf", "config:hypr/ignored-extra.conf"}},
	}
	r := &fakeRunner{}
	svc := newService(t, in, r)
	hp, err := PlanHard(context.Background(), svc, dir, []manifest.ResolvedEntry{hyprGPU, gone})
	if err != nil {
		t.Fatalf("PlanHard: %v", err)
	}
	return hp, svc, r
}

var wantHardSelected = []string{"a.conf", "b.conf", "extra.conf", "old", "old/f.txt"}

func TestPlanHard_SelectsEveryNonBlockedNonIgnoredItemDeletionsIncluded(t *testing.T) {
	for _, dir := range []plan.Direction{plan.Push, plan.Pull} {
		hp, _, _ := hardFixture(t, dir)

		var got []string
		for _, it := range hp.Items {
			if hp.Selection[it.Key()] {
				got = append(got, it.Change.Path)
			}
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, wantHardSelected) {
			t.Errorf("direction %v: selected = %v, want %v", dir, got, wantHardSelected)
		}
		if hp.Direction != dir {
			t.Errorf("direction %v: HardPlan.Direction = %v", dir, hp.Direction)
		}
		if !reflect.DeepEqual(hp.Selection, hp.SyncPlan.Selection) {
			t.Errorf("direction %v: HardPlan.Selection and SyncPlan.Selection differ", dir)
		}
	}
}

func TestPlanHard_ExcludedPathIsNeverPlannedListedOrDeleted(t *testing.T) {
	for _, dir := range []plan.Direction{plan.Push, plan.Pull} {
		hp, svc, r := hardFixture(t, dir)
		for _, it := range hp.Items {
			if strings.HasSuffix(it.Change.Path, "gpu_env.lua") {
				t.Errorf("direction %v: excluded path is in the plan: %+v", dir, it)
			}
		}
		if strings.Contains(hp.Confirm, "gpu_env.lua") {
			t.Errorf("direction %v: confirm mentions the excluded path:\n%s", dir, hp.Confirm)
		}
		svc.Apply(context.Background(), hp.SyncPlan, hp.Selection, hardOpts, nil)
		for _, d := range stepsOfType[plan.DeleteStep](r.steps) {
			if strings.HasSuffix(d.Path, "gpu_env.lua") {
				t.Errorf("direction %v: excluded path deleted: %s", dir, d.Path)
			}
		}
	}
}

func TestPlanHard_EntryMissingOnTheSourceSideIsSkippedWithNoticeNotWiped(t *testing.T) {
	for _, dir := range []plan.Direction{plan.Push, plan.Pull} {
		hp, _, _ := hardFixture(t, dir)
		for _, it := range hp.Items {
			if it.Resolved.Path == "gone" {
				t.Errorf("direction %v: missing-source entry has plan items: %+v", dir, it)
			}
		}
		if !strings.Contains(strings.Join(hp.Notices, "\n"), "skipped: source missing: config/gone") {
			t.Errorf("direction %v: notices = %v", dir, hp.Notices)
		}
		if !strings.Contains(hp.Confirm, "skipped: source missing: config/gone") {
			t.Errorf("direction %v: confirm does not list the skipped entry:\n%s", dir, hp.Confirm)
		}
	}
}

func TestPlanHard_ConfirmNamesDirectionDeletionsAndWhatIsNotTouched(t *testing.T) {
	tests := []struct {
		dir          plan.Direction
		header       string
		deletedFrom  string
		notDeletedAt string
	}{
		{plan.Push, "Hard reinstall: repo -> PC", "deleting from the pc", "deleting from the repo"},
		{plan.Pull, "Hard reinstall: PC -> repo", "deleting from the repo", "deleting from the pc"},
	}
	for _, tt := range tests {
		hp, _, _ := hardFixture(t, tt.dir)
		lines := map[string]bool{}
		var all []string
		for _, l := range strings.Split(hp.Confirm, "\n") {
			l = strings.TrimSpace(l)
			lines[l] = true
			all = append(all, l)
		}
		lower := strings.ToLower(hp.Confirm)

		if all[0] != tt.header {
			t.Errorf("%s: first line = %q", tt.header, all[0])
		}
		for _, want := range []string{"delete config:hypr/extra.conf", "delete config:hypr/old (directory)", "delete config:hypr/old/f.txt", "Overwrite or create: 2"} {
			if !lines[want] {
				t.Errorf("%s: confirm lacks line %q:\n%s", tt.header, want, hp.Confirm)
			}
		}
		if strings.Contains(hp.Confirm, "delete config:hypr/ignored-extra.conf") {
			t.Errorf("%s: ignored deletion is listed as a deletion", tt.header)
		}
		if !strings.Contains(lower, tt.deletedFrom) || strings.Contains(lower, tt.notDeletedAt) {
			t.Errorf("%s: confirm must name only %q:\n%s", tt.header, tt.deletedFrom, hp.Confirm)
		}

		at := strings.Index(hp.Confirm, notTouchedHeader)
		if at < 0 {
			t.Fatalf("%s: confirm lacks %q:\n%s", tt.header, notTouchedHeader, hp.Confirm)
		}
		before, after := hp.Confirm[:at], hp.Confirm[at:]
		for _, key := range []string{"config:hypr/ignored.conf", "config:hypr/ignored-extra.conf", "config:hypr/sub/c.conf"} {
			if !strings.Contains(after, key) {
				t.Errorf("%s: %s missing under %q", tt.header, key, notTouchedHeader)
			}
			if strings.Contains(before, key) {
				t.Errorf("%s: %s listed outside the not-touched section", tt.header, key)
			}
		}
	}
}

func TestPlanHard_ApplyDeletesOnlyTheConfirmedListAndNeverTouchesProtectedPaths(t *testing.T) {
	for _, dir := range []plan.Direction{plan.Push, plan.Pull} {
		hp, svc, r := hardFixture(t, dir)
		res := svc.Apply(context.Background(), hp.SyncPlan, hp.Selection, hardOpts, nil)
		if res.Err != nil {
			t.Fatalf("direction %v: Apply: %v", dir, res.Err)
		}

		base := hp.Items[0].Dst + "/"
		var deleted []string
		for _, d := range stepsOfType[plan.DeleteStep](r.steps) {
			deleted = append(deleted, d.Path)
		}
		wantDeleted := []string{base + "extra.conf", base + "old/f.txt", base + "old"}
		if !reflect.DeepEqual(deleted, wantDeleted) {
			t.Errorf("direction %v: deleted = %v, want %v", dir, deleted, wantDeleted)
		}

		var copied []string
		for _, c := range stepsOfType[plan.CopyStep](r.steps) {
			copied = append(copied, c.Paths...)
		}
		if !reflect.DeepEqual(copied, []string{"a.conf", "b.conf"}) {
			t.Errorf("direction %v: copied = %v, want only the selected non-protected files", dir, copied)
		}
	}
}
