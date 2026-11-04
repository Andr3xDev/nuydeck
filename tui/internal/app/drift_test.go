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
)

func collectDrift(svc *Service, entries []domain.ResolvedEntry) map[string]DriftResult {
	out := map[string]DriftResult{}
	for r := range svc.Drift(context.Background(), entries) {
		out[r.Entry.Path] = r
	}
	return out
}

func TestDrift_ClassifiesEntriesAndIsolatesErrors(t *testing.T) {
	boom := errors.New("rsync exited with code 23")
	in := &fakeInspector{diff: func(e domain.ResolvedEntry, src, dst domain.Endpoint) ([]domain.Change, error) {
		switch e.Path {
		case "same":
			return nil, nil
		case "bad":
			return nil, boom
		}
		return []domain.Change{
			change("repo-only", domain.Create, domain.KindFile),
			change("pc-only", domain.Delete, domain.KindFile),
			change("differs", domain.Update, domain.KindFile),
		}, nil
	}}
	entries := []domain.ResolvedEntry{
		resolved(domain.RootConfig, "same"), resolved(domain.RootConfig, "bad"), resolved(domain.RootConfig, "drifty"),
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

func TestDrift_RunsAtMostFourDiffsAtOnce(t *testing.T) {
	var inFlight, peak atomic.Int32
	in := &fakeInspector{diff: func(domain.ResolvedEntry, domain.Endpoint, domain.Endpoint) ([]domain.Change, error) {
		n := inFlight.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		return nil, nil
	}}
	var entries []domain.ResolvedEntry
	for i := range 12 {
		entries = append(entries, resolved(domain.RootConfig, fmt.Sprintf("e%d", i)))
	}
	if got := collectDrift(newService(t, in, &fakeRunner{}), entries); len(got) != 12 {
		t.Fatalf("results = %d, want 12", len(got))
	}
	if peak.Load() > DriftConcurrency {
		t.Fatalf("peak concurrency = %d, want <= %d", peak.Load(), DriftConcurrency)
	}
}

func TestDrift_AttachesTimesAndNewestCompleteBackup(t *testing.T) {
	e := resolved(domain.RootConfig, "hypr")
	pcT, repoT := testNow.Add(-time.Hour), testNow.Add(-72*time.Hour)
	complete := domain.Backup{Dir: "/home/u/.nuydeck-backups/20261103-100000", Created: testNow.Add(-26 * time.Hour)}
	in := &fakeInspector{
		diff:      diffReturning(),
		mtimes:    map[string]time.Time{e.DestPath: pcT},
		repoTimes: map[string]domain.RepoTimes{"config/hypr": {Entry: repoT}},
		backups:   []domain.Backup{{Dir: "/home/u/.nuydeck-backups/x.partial", Incomplete: true}, complete},
	}
	r := collectDrift(newService(t, in, &fakeRunner{}), []domain.ResolvedEntry{e})["hypr"]

	if !r.PCTime.Equal(pcT) || !r.RepoTimes.Entry.Equal(repoT) {
		t.Errorf("times pc=%v repo=%v", r.PCTime, r.RepoTimes.Entry)
	}
	if r.LastBackup == nil || r.LastBackup.Dir != complete.Dir {
		t.Errorf("LastBackup = %+v, want %s", r.LastBackup, complete.Dir)
	}
}

func testBackup() domain.Backup {
	return domain.Backup{
		Dir:       "/home/u/.nuydeck-backups/20261103-100000",
		Direction: domain.Push,
		Roots:     map[domain.Root]string{domain.RootConfig: "/home/u/.config"},
		Files:     []string{"config/hypr/a.conf"},
	}
}

func TestPlanRestore_DiffsBackupIntoRecordedRootsAndNeverDeletes(t *testing.T) {
	b := testBackup()
	in := &fakeInspector{diff: diffReturning(
		change("hypr/a.conf", domain.Update, domain.KindFile),
		change("hypr/b.conf", domain.Create, domain.KindFile),
		change("hypr/extra.conf", domain.Delete, domain.KindFile),
	)}
	plan, err := newService(t, in, &fakeRunner{}).PlanRestore(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Direction != domain.Restore {
		t.Errorf("direction = %v, want Restore", plan.Direction)
	}
	if len(in.diffCalls) != 1 || in.diffCalls[0] != (diffCall{b.Dir + "/config", "/home/u/.config"}) {
		t.Errorf("diff calls = %+v", in.diffCalls)
	}
	for _, it := range plan.Items {
		if it.Change.Type == domain.Delete {
			t.Errorf("restore offered deletion of %s", it.Change.Path)
		}
	}
	if len(plan.Items) != 2 {
		t.Errorf("items = %d, want 2", len(plan.Items))
	}
}

func TestApply_RestoreCopiesFromBackupAndOffersBackupForOverwrite(t *testing.T) {
	b := testBackup()
	in := &fakeInspector{diff: diffReturning(change("hypr/a.conf", domain.Update, domain.KindFile))}
	svc := newService(t, in, &fakeRunner{})
	plan, err := svc.PlanRestore(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	sel := selectAll(plan)
	if !svc.NeedsBackup(plan, sel) {
		t.Error("restore overwriting a file must offer a backup")
	}
	r := &fakeRunner{}
	newService(t, in, r).Apply(context.Background(), plan, sel, ApplyOptions{Backup: true}, nil)

	copies := stepsOfType[domain.CopyStep](r.steps)
	if len(copies) != 1 || copies[0].Direction != domain.Restore || copies[0].Src != b.Dir+"/config" ||
		copies[0].Dst != "/home/u/.config" || len(copies[0].Paths) != 1 || copies[0].Paths[0] != "hypr/a.conf" {
		t.Fatalf("copy steps = %+v", copies)
	}
	if len(stepsOfType[domain.DeleteStep](r.steps)) != 0 {
		t.Error("restore must not delete")
	}
}

func TestDeleteBackup_DeclinedRunsNoStep(t *testing.T) {
	dir := "/home/u/.nuydeck-backups/20261103-100000"
	tests := []struct {
		name      string
		confirmed bool
		wantSteps []domain.Step
	}{
		{"declined", false, nil},
		{"confirmed", true, []domain.Step{domain.DeleteBackupStep{Dir: dir}}},
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
