package cfgsync

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

func TestPlanSync_DiffDirectionFollowsSyncDirection(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	tests := []struct {
		name         string
		dir          plan.Direction
		wantSrc, dst string
	}{
		{"push copies repo to pc", plan.Push, e.RepoPath, e.DestPath},
		{"pull copies pc to repo", plan.Pull, e.DestPath, e.RepoPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &fakeInspector{diff: diffReturning()}
			if _, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), tt.dir, []manifest.ResolvedEntry{e}); err != nil {
				t.Fatal(err)
			}
			if len(in.diffCalls) != 1 || in.diffCalls[0] != (diffCall{tt.wantSrc, tt.dst}) {
				t.Fatalf("diff calls = %+v, want one %s -> %s", in.diffCalls, tt.wantSrc, tt.dst)
			}
		})
	}
}

func TestPlanSync_MissingSourceSkipsEntryAndPlanContinues(t *testing.T) {
	gone, ok := resolved(manifest.RootConfig, "gone"), resolved(manifest.RootConfig, "ok")
	in := &fakeInspector{
		diff:  diffReturning(change("a.conf", plan.Create, plan.KindFile)),
		kinds: map[string]plan.NodeKind{gone.RepoPath: plan.KindNone},
	}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{gone, ok})
	if err != nil {
		t.Fatalf("missing source must not fail the plan: %v", err)
	}
	if len(syncPlan.Items) != 1 || syncPlan.Items[0].Resolved.Path != "ok" {
		t.Fatalf("items = %+v, want only the ok entry", syncPlan.Items)
	}
	for _, c := range in.diffCalls {
		if c.src == gone.RepoPath {
			t.Errorf("diff must not run for a missing source: %+v", c)
		}
	}
	// The notice field shape is a design choice; match on the rendered plan.
	if !strings.Contains(fmt.Sprintf("%+v", syncPlan), "skipped: source missing") {
		t.Errorf("plan carries no %q notice: %+v", "skipped: source missing", syncPlan)
	}
}

func TestPlanSync_RecomputesOnEveryCall(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	calls := 0
	in := &fakeInspector{diff: func(manifest.ResolvedEntry, plan.Endpoint, plan.Endpoint) ([]plan.Change, error) {
		calls++
		if calls == 1 {
			return []plan.Change{change("a.conf", plan.Update, plan.KindFile)}, nil
		}
		return nil, nil
	}}
	svc := newService(t, in, &fakeRunner{})
	first, _ := svc.PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	second, _ := svc.PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if len(first.Items) != 1 || len(second.Items) != 0 || calls != 2 {
		t.Fatalf("items %d then %d after %d diffs; want 1 then 0 after 2", len(first.Items), len(second.Items), calls)
	}
}

func TestPlanSync_ExcludedChangesAreNeverOffered(t *testing.T) {
	e := resolved(manifest.RootConfig, "nuydeck", "/logs/")
	in := &fakeInspector{diff: diffReturning(
		change("logs/2026-11.jsonl", plan.Create, plan.KindFile),
		change("a.conf", plan.Create, plan.KindFile),
	)}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	if len(syncPlan.Items) != 1 || syncPlan.Items[0].Change.Path != "a.conf" {
		t.Fatalf("items = %+v, want only a.conf", syncPlan.Items)
	}
}

func TestPlanSync_LinkWarningsUseSourceSideTarget(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	in := &fakeInspector{
		diff: diffReturning(
			change("abs.lnk", plan.Create, plan.KindLink),
			change("out.lnk", plan.Create, plan.KindLink),
			change("in.lnk", plan.Create, plan.KindLink),
		),
		linkTargets: map[string]string{
			e.RepoPath + "/abs.lnk": "/etc/hosts",
			e.RepoPath + "/out.lnk": "../other/file",
			e.RepoPath + "/in.lnk":  "sub/file",
		},
	}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]plan.LinkWarning{
		"abs.lnk": plan.WarnAbsoluteLink,
		"out.lnk": plan.WarnLeavesManagedSet,
		"in.lnk":  plan.NoLinkWarning,
	}
	for _, it := range syncPlan.Items {
		if got := it.Change.Warning; got != want[it.Change.Path] {
			t.Errorf("%s warning = %v, want %v", it.Change.Path, got, want[it.Change.Path])
		}
	}
}

func TestPlanSync_ItemBelowSymlinkedParentIsBlockedAndNotSelected(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	in := &fakeInspector{
		diff:  diffReturning(change("sub/a.conf", plan.Create, plan.KindFile), change("b.conf", plan.Create, plan.KindFile)),
		kinds: map[string]plan.NodeKind{e.DestPath + "/sub": plan.KindLink},
	}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	blocked := map[string]bool{}
	for _, it := range syncPlan.Items {
		blocked[it.Change.Path] = it.Blocked
		if it.Blocked && syncPlan.Selection[it.Key()] {
			t.Errorf("blocked item %s is selected", it.Change.Path)
		}
	}
	if !blocked["sub/a.conf"] || blocked["b.conf"] {
		t.Fatalf("blocked = %v, want only sub/a.conf", blocked)
	}
}

func TestPlanSync_DefaultSelectionIsAllExceptStateIgnores(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	in := &fakeInspector{
		diff:  diffReturning(change("a.conf", plan.Update, plan.KindFile), change("b.conf", plan.Update, plan.KindFile)),
		state: state.State{Ignores: []string{"config:hypr/b.conf"}},
	}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, it := range syncPlan.Items {
		selected[it.Change.Path] = syncPlan.Selection[it.Key()]
	}
	if !selected["a.conf"] || selected["b.conf"] {
		t.Fatalf("selected = %v, want a.conf only", selected)
	}
}

func TestPlanSync_CorruptStateWarningIsCarriedNotFatal(t *testing.T) {
	in := &fakeInspector{diff: diffReturning(), stateWarn: "state.toml is corrupt, using defaults"}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{resolved(manifest.RootConfig, "hypr")})
	if err != nil || syncPlan.StateWarning != in.stateWarn {
		t.Fatalf("err = %v, warning = %q", err, syncPlan.StateWarning)
	}
}

func TestPlanSync_LastUpdatedShowsOnlySidesThatHaveTheFile(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	repoT, pcT := testNow.Add(-48*time.Hour), testNow.Add(-2*time.Hour)
	in := &fakeInspector{
		diff: diffReturning(
			change("upd.conf", plan.Update, plan.KindFile),
			change("new.conf", plan.Create, plan.KindFile),
			change("old.conf", plan.Delete, plan.KindFile),
		),
		repoTimes: map[string]plan.RepoTimes{"config/hypr": {Files: map[string]time.Time{"upd.conf": repoT, "new.conf": repoT}}},
		mtimes:    map[string]time.Time{e.DestPath + "/upd.conf": pcT, e.DestPath + "/old.conf": pcT},
	}
	syncPlan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	type times struct{ repo, pc time.Time }
	want := map[string]times{
		"upd.conf": {repoT, pcT},
		"new.conf": {repoT, time.Time{}},
		"old.conf": {time.Time{}, pcT},
	}
	for _, it := range syncPlan.Items {
		if got := (times{it.RepoTime, it.PCTime}); got != want[it.Change.Path] {
			t.Errorf("%s times = %+v, want %+v", it.Change.Path, got, want[it.Change.Path])
		}
	}
}
