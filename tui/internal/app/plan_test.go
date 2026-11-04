package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func TestPlanSync_DiffDirectionFollowsSyncDirection(t *testing.T) {
	e := resolved(domain.RootConfig, "hypr")
	tests := []struct {
		name         string
		dir          domain.Direction
		wantSrc, dst string
	}{
		{"push copies repo to pc", domain.Push, e.RepoPath, e.DestPath},
		{"pull copies pc to repo", domain.Pull, e.DestPath, e.RepoPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &fakeInspector{diff: diffReturning()}
			if _, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), tt.dir, []domain.ResolvedEntry{e}); err != nil {
				t.Fatal(err)
			}
			if len(in.diffCalls) != 1 || in.diffCalls[0] != (diffCall{tt.wantSrc, tt.dst}) {
				t.Fatalf("diff calls = %+v, want one %s -> %s", in.diffCalls, tt.wantSrc, tt.dst)
			}
		})
	}
}

func TestPlanSync_MissingSourceSkipsEntryAndPlanContinues(t *testing.T) {
	gone, ok := resolved(domain.RootConfig, "gone"), resolved(domain.RootConfig, "ok")
	in := &fakeInspector{
		diff:  diffReturning(change("a.conf", domain.Create, domain.KindFile)),
		kinds: map[string]domain.NodeKind{gone.RepoPath: domain.KindNone},
	}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{gone, ok})
	if err != nil {
		t.Fatalf("missing source must not fail the plan: %v", err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Resolved.Path != "ok" {
		t.Fatalf("items = %+v, want only the ok entry", plan.Items)
	}
	for _, c := range in.diffCalls {
		if c.src == gone.RepoPath {
			t.Errorf("diff must not run for a missing source: %+v", c)
		}
	}
	// The notice field shape is a design choice; match on the rendered plan.
	if !strings.Contains(fmt.Sprintf("%+v", plan), "skipped: source missing") {
		t.Errorf("plan carries no %q notice: %+v", "skipped: source missing", plan)
	}
}

func TestPlanSync_RecomputesOnEveryCall(t *testing.T) {
	e := resolved(domain.RootConfig, "hypr")
	calls := 0
	in := &fakeInspector{diff: func(domain.ResolvedEntry, domain.Endpoint, domain.Endpoint) ([]domain.Change, error) {
		calls++
		if calls == 1 {
			return []domain.Change{change("a.conf", domain.Update, domain.KindFile)}, nil
		}
		return nil, nil
	}}
	svc := newService(t, in, &fakeRunner{})
	first, _ := svc.PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{e})
	second, _ := svc.PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{e})
	if len(first.Items) != 1 || len(second.Items) != 0 || calls != 2 {
		t.Fatalf("items %d then %d after %d diffs; want 1 then 0 after 2", len(first.Items), len(second.Items), calls)
	}
}

func TestPlanSync_ExcludedChangesAreNeverOffered(t *testing.T) {
	e := resolved(domain.RootConfig, "nuydeck", "/logs/")
	in := &fakeInspector{diff: diffReturning(
		change("logs/2026-11.jsonl", domain.Create, domain.KindFile),
		change("a.conf", domain.Create, domain.KindFile),
	)}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Change.Path != "a.conf" {
		t.Fatalf("items = %+v, want only a.conf", plan.Items)
	}
}

func TestPlanSync_LinkWarningsUseSourceSideTarget(t *testing.T) {
	e := resolved(domain.RootConfig, "hypr")
	in := &fakeInspector{
		diff: diffReturning(
			change("abs.lnk", domain.Create, domain.KindLink),
			change("out.lnk", domain.Create, domain.KindLink),
			change("in.lnk", domain.Create, domain.KindLink),
		),
		linkTargets: map[string]string{
			e.RepoPath + "/abs.lnk": "/etc/hosts",
			e.RepoPath + "/out.lnk": "../other/file",
			e.RepoPath + "/in.lnk":  "sub/file",
		},
	}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]domain.LinkWarning{
		"abs.lnk": domain.WarnAbsoluteLink,
		"out.lnk": domain.WarnLeavesManagedSet,
		"in.lnk":  domain.NoLinkWarning,
	}
	for _, it := range plan.Items {
		if got := it.Change.Warning; got != want[it.Change.Path] {
			t.Errorf("%s warning = %v, want %v", it.Change.Path, got, want[it.Change.Path])
		}
	}
}

func TestPlanSync_ItemBelowSymlinkedParentIsBlockedAndNotSelected(t *testing.T) {
	e := resolved(domain.RootConfig, "hypr")
	in := &fakeInspector{
		diff:  diffReturning(change("sub/a.conf", domain.Create, domain.KindFile), change("b.conf", domain.Create, domain.KindFile)),
		kinds: map[string]domain.NodeKind{e.DestPath + "/sub": domain.KindLink},
	}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	blocked := map[string]bool{}
	for _, it := range plan.Items {
		blocked[it.Change.Path] = it.Blocked
		if it.Blocked && plan.Selection[it.Key()] {
			t.Errorf("blocked item %s is selected", it.Change.Path)
		}
	}
	if !blocked["sub/a.conf"] || blocked["b.conf"] {
		t.Fatalf("blocked = %v, want only sub/a.conf", blocked)
	}
}

func TestPlanSync_DefaultSelectionIsAllExceptStateIgnores(t *testing.T) {
	e := resolved(domain.RootConfig, "hypr")
	in := &fakeInspector{
		diff:  diffReturning(change("a.conf", domain.Update, domain.KindFile), change("b.conf", domain.Update, domain.KindFile)),
		state: domain.State{Ignores: []string{"config:hypr/b.conf"}},
	}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, it := range plan.Items {
		selected[it.Change.Path] = plan.Selection[it.Key()]
	}
	if !selected["a.conf"] || selected["b.conf"] {
		t.Fatalf("selected = %v, want a.conf only", selected)
	}
}

func TestPlanSync_CorruptStateWarningIsCarriedNotFatal(t *testing.T) {
	in := &fakeInspector{diff: diffReturning(), stateWarn: "state.toml is corrupt, using defaults"}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{resolved(domain.RootConfig, "hypr")})
	if err != nil || plan.StateWarning != in.stateWarn {
		t.Fatalf("err = %v, warning = %q", err, plan.StateWarning)
	}
}

func TestPlanSync_LastUpdatedShowsOnlySidesThatHaveTheFile(t *testing.T) {
	e := resolved(domain.RootConfig, "hypr")
	repoT, pcT := testNow.Add(-48*time.Hour), testNow.Add(-2*time.Hour)
	in := &fakeInspector{
		diff: diffReturning(
			change("upd.conf", domain.Update, domain.KindFile),
			change("new.conf", domain.Create, domain.KindFile),
			change("old.conf", domain.Delete, domain.KindFile),
		),
		repoTimes: map[string]domain.RepoTimes{"config/hypr": {Files: map[string]time.Time{"upd.conf": repoT, "new.conf": repoT}}},
		mtimes:    map[string]time.Time{e.DestPath + "/upd.conf": pcT, e.DestPath + "/old.conf": pcT},
	}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), domain.Push, []domain.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	type times struct{ repo, pc time.Time }
	want := map[string]times{
		"upd.conf": {repoT, pcT},
		"new.conf": {repoT, time.Time{}},
		"old.conf": {time.Time{}, pcT},
	}
	for _, it := range plan.Items {
		if got := (times{it.RepoTime, it.PCTime}); got != want[it.Change.Path] {
			t.Errorf("%s times = %+v, want %+v", it.Change.Path, got, want[it.Change.Path])
		}
	}
}
