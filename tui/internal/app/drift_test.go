package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func collectDrift(svc *Service, entries []manifest.ResolvedEntry) map[string]DriftResult {
	out := map[string]DriftResult{}
	for r := range svc.Drift(context.Background(), entries) {
		out[r.Entry.Path] = r
	}
	return out
}

func TestDrift_ClassifiesEntriesAndIsolatesErrors(t *testing.T) {
	boom := errors.New("rsync exited with code 23")
	in := &fakeInspector{diff: func(e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error) {
		switch e.Path {
		case "same":
			return nil, nil
		case "bad":
			return nil, boom
		}
		return []plan.Change{
			change("repo-only", plan.Create, plan.KindFile),
			change("pc-only", plan.Delete, plan.KindFile),
			change("differs", plan.Update, plan.KindFile),
		}, nil
	}}
	entries := []manifest.ResolvedEntry{
		resolved(manifest.RootConfig, "same"), resolved(manifest.RootConfig, "bad"), resolved(manifest.RootConfig, "drifty"),
	}
	got := collectDrift(newService(t, in, &fakeRunner{}), entries)

	if len(got) != 3 {
		t.Fatalf("results = %d, want 3", len(got))
	}
	if got["same"].State != DriftInSync {
		t.Errorf("same state = %v", got["same"].State)
	}
	if got["bad"].State != DriftError || !errors.Is(got["bad"].Err, boom) {
		t.Errorf("bad = %+v", got["bad"])
	}
	if got["drifty"].State != DriftDrifted || got["drifty"].Changes != 3 {
		t.Errorf("drifty = %+v", got["drifty"])
	}
	for _, c := range in.diffCalls {
		if !strings.HasPrefix(c.src, testRepo+"/") || !strings.HasPrefix(c.dst, testHome+"/") {
			t.Errorf("diff must run repo -> pc once per entry: %+v", c)
		}
	}
	if len(in.diffCalls) != 3 {
		t.Errorf("diff calls = %d, want one per entry", len(in.diffCalls))
	}
}

func TestDrift_MissingSideIsClassifiedWithoutDiff(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	tests := []struct {
		name        string
		kinds       map[string]plan.NodeKind
		wantState   DriftState
		wantMissing Missing
		wantErr     string
	}{
		{"absent in repo", map[string]plan.NodeKind{e.RepoPath: plan.KindNone, e.DestPath: plan.KindDir}, DriftDrifted, MissingInRepo, ""},
		{"absent on pc", map[string]plan.NodeKind{e.RepoPath: plan.KindDir, e.DestPath: plan.KindNone}, DriftDrifted, MissingOnPC, ""},
		{"absent on both", map[string]plan.NodeKind{e.RepoPath: plan.KindNone, e.DestPath: plan.KindNone}, DriftError, MissingNone, "absent on both sides"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &fakeInspector{diff: diffReturning(), kinds: tt.kinds}
			r := collectDrift(newService(t, in, &fakeRunner{}), []manifest.ResolvedEntry{e})["hypr"]

			if r.State != tt.wantState || r.Missing != tt.wantMissing {
				t.Errorf("state = %v missing = %v, want %v %v", r.State, r.Missing, tt.wantState, tt.wantMissing)
			}
			if tt.wantErr != "" && (r.Err == nil || !strings.Contains(r.Err.Error(), tt.wantErr)) {
				t.Errorf("err = %v, want %q", r.Err, tt.wantErr)
			}
			if len(in.diffCalls) != 0 {
				t.Errorf("diff must not run when a side is absent: %+v", in.diffCalls)
			}
		})
	}
}

func TestDrift_RunsAtMostFourDiffsAtOnce(t *testing.T) {
	var inFlight, peak atomic.Int32
	in := &fakeInspector{diff: func(manifest.ResolvedEntry, plan.Endpoint, plan.Endpoint) ([]plan.Change, error) {
		n := inFlight.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		return nil, nil
	}}
	var entries []manifest.ResolvedEntry
	for i := range 12 {
		entries = append(entries, resolved(manifest.RootConfig, fmt.Sprintf("e%d", i)))
	}
	if got := collectDrift(newService(t, in, &fakeRunner{}), entries); len(got) != 12 {
		t.Fatalf("results = %d, want 12", len(got))
	}
	if peak.Load() > DriftConcurrency {
		t.Fatalf("peak concurrency = %d, want <= %d", peak.Load(), DriftConcurrency)
	}
}

func TestDrift_AttachesTimesAndNewestCompleteBackup(t *testing.T) {
	e := resolved(manifest.RootConfig, "hypr")
	pcT, repoT := testNow.Add(-time.Hour), testNow.Add(-72*time.Hour)
	complete := domain.Backup{Dir: "/home/u/.nuydeck-backups/20261103-100000", Created: testNow.Add(-26 * time.Hour)}
	in := &fakeInspector{
		diff:      diffReturning(),
		mtimes:    map[string]time.Time{e.DestPath: pcT},
		repoTimes: map[string]plan.RepoTimes{"config/hypr": {Entry: repoT}},
		backups:   []domain.Backup{{Dir: "/home/u/.nuydeck-backups/x.partial", Incomplete: true}, complete},
	}
	r := collectDrift(newService(t, in, &fakeRunner{}), []manifest.ResolvedEntry{e})["hypr"]

	if !r.PCTime.Equal(pcT) || !r.RepoTimes.Entry.Equal(repoT) {
		t.Errorf("times pc=%v repo=%v", r.PCTime, r.RepoTimes.Entry)
	}
	if r.LastBackup == nil || r.LastBackup.Dir != complete.Dir {
		t.Errorf("LastBackup = %+v, want %s", r.LastBackup, complete.Dir)
	}
}

const driftWait = 2 * time.Second

func TestDrift_CancelledContextYieldsOnlyErrorsAndClosesTheChannel(t *testing.T) {
	var entries []manifest.ResolvedEntry
	for i := range 3 * DriftConcurrency {
		entries = append(entries, resolved(manifest.RootConfig, fmt.Sprintf("e%d", i)))
	}
	in := &fakeInspector{diff: diffReturning()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := newService(t, in, &fakeRunner{}).Drift(ctx, entries)

	timeout := time.After(driftWait)
	for {
		select {
		case r, open := <-ch:
			if !open {
				return
			}
			if r.State != DriftError || !errors.Is(r.Err, context.Canceled) {
				t.Fatalf("result = %+v, want DriftError wrapping context.Canceled", r)
			}
		case <-timeout:
			t.Fatal("channel not closed after cancelled context")
		}
	}
}

func TestDrift_ReturnsBeforeListBackupsFinishes(t *testing.T) {
	gate := make(chan struct{})
	in := &fakeInspector{diff: diffReturning(), listBackupsGate: gate}
	svc := newService(t, in, &fakeRunner{})
	returned := make(chan (<-chan DriftResult), 1)
	go func() {
		returned <- svc.Drift(context.Background(), []manifest.ResolvedEntry{resolved(manifest.RootConfig, "a")})
	}()

	select {
	case ch := <-returned:
		close(gate)
		for range ch {
		}
	case <-time.After(driftWait):
		close(gate)
		t.Fatal("Drift blocked on ListBackups before returning the channel")
	}
}

func testBackup() domain.Backup {
	return domain.Backup{
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
	newService(t, in, r).Apply(context.Background(), syncPlan, sel, ApplyOptions{Backup: true}, nil)

	copies := stepsOfType[plan.CopyStep](r.steps)
	if len(copies) != 1 || copies[0].Direction != plan.Restore || copies[0].Src != b.Dir+"/config" ||
		copies[0].Dst != "/home/u/.config" || len(copies[0].Paths) != 1 || copies[0].Paths[0] != "hypr/a.conf" {
		t.Fatalf("copy steps = %+v", copies)
	}
	if len(stepsOfType[plan.DeleteStep](r.steps)) != 0 {
		t.Error("restore must not delete")
	}
}

func TestDeleteBackup_DeclinedRunsNoStep(t *testing.T) {
	dir := "/home/u/.nuydeck-backups/20261103-100000"
	tests := []struct {
		name      string
		confirmed bool
		wantSteps []plan.Step
	}{
		{"declined", false, nil},
		{"confirmed", true, []plan.Step{plan.DeleteBackupStep{Dir: dir}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{}
			_, ran := newService(t, &fakeInspector{}, r).DeleteBackup(context.Background(), dir, tt.confirmed)
			if ran != tt.confirmed || len(r.steps) != len(tt.wantSteps) || (ran && r.steps[0] != tt.wantSteps[0]) {
				t.Fatalf("ran = %v, steps = %v, want %v", ran, r.steps, tt.wantSteps)
			}
		})
	}
}
