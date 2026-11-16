package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const (
	testRepo = "/repo"
	testHome = "/home/u"
)

var testNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

var rootDirs = map[manifest.Root]string{
	manifest.RootConfig: testHome + "/.config",
	manifest.RootHome:   testHome,
}

type diffCall struct{ src, dst string }

type fakeInspector struct {
	mu          sync.Mutex
	diff        func(e manifest.ResolvedEntry, src, dst domain.Endpoint) ([]domain.Change, error)
	diffCalls   []diffCall
	linkTargets map[string]string
	kinds       map[string]domain.NodeKind
	mtimes      map[string]time.Time
	repoTimes   map[string]domain.RepoTimes
	backups     []domain.Backup
	state       state.State
	stateWarn   string
	// listBackupsGate, when set, blocks ListBackups until it is closed.
	listBackupsGate chan struct{}
}

func (f *fakeInspector) Diff(_ context.Context, e manifest.ResolvedEntry, src, dst domain.Endpoint) ([]domain.Change, error) {
	f.mu.Lock()
	f.diffCalls = append(f.diffCalls, diffCall{src.Path, dst.Path})
	f.mu.Unlock()
	return f.diff(e, src, dst)
}

func (f *fakeInspector) LinkTarget(p string) (string, error) { return f.linkTargets[p], nil }

func (f *fakeInspector) PathKind(p string) (domain.NodeKind, error) {
	if k, ok := f.kinds[p]; ok {
		return k, nil
	}
	return domain.KindDir, nil
}

func (f *fakeInspector) NewestMTime(_ context.Context, p string, _ manifest.Filter) (time.Time, error) {
	return f.mtimes[p], nil
}

func (f *fakeInspector) RepoTimes(_ context.Context, _, rel string) (domain.RepoTimes, error) {
	return f.repoTimes[rel], nil
}

func (f *fakeInspector) ListBackups(context.Context) ([]domain.Backup, error) {
	if f.listBackupsGate != nil {
		<-f.listBackupsGate
	}
	return f.backups, nil
}

func (f *fakeInspector) LoadState(context.Context) (state.State, string) {
	return f.state, f.stateWarn
}

type fakeRunner struct {
	mu     sync.Mutex
	steps  []domain.Step
	result func(domain.Step) domain.StepResult
}

func (r *fakeRunner) Run(_ context.Context, s domain.Step, progress func(domain.StepProgress)) domain.StepResult {
	r.mu.Lock()
	r.steps = append(r.steps, s)
	r.mu.Unlock()
	if progress != nil {
		progress(domain.StepProgress{Path: "tick"})
	}
	if r.result != nil {
		return r.result(s)
	}
	return domain.StepResult{Status: domain.StepOK}
}

func resolved(root manifest.Root, p string, excludes ...string) manifest.ResolvedEntry {
	return manifest.ResolvedEntry{
		Entry:    manifest.Entry{Root: root, Path: p},
		RepoPath: testRepo + "/" + string(root) + "/" + p,
		DestPath: rootDirs[root] + "/" + p,
		Filter:   manifest.Filter{Excludes: excludes},
	}
}

func newService(t *testing.T, in *fakeInspector, r Runner) *Service {
	t.Helper()
	return &Service{Inspector: in, Runner: r, RepoDir: testRepo, Now: func() time.Time { return testNow }}
}

func diffReturning(changes ...domain.Change) func(manifest.ResolvedEntry, domain.Endpoint, domain.Endpoint) ([]domain.Change, error) {
	return func(manifest.ResolvedEntry, domain.Endpoint, domain.Endpoint) ([]domain.Change, error) {
		return changes, nil
	}
}

func change(p string, t domain.ChangeType, k domain.NodeKind) domain.Change {
	return domain.Change{Path: p, Type: t, Kind: k}
}

func selectAll(plan SyncPlan) Selection {
	sel := Selection{}
	for _, it := range plan.Items {
		sel[it.Key()] = true
	}
	return sel
}

func planOf(t *testing.T, dir domain.Direction, entries []manifest.ResolvedEntry, byEntry map[string][]domain.Change) SyncPlan {
	t.Helper()
	in := &fakeInspector{diff: func(e manifest.ResolvedEntry, _, _ domain.Endpoint) ([]domain.Change, error) {
		return byEntry[e.Path], nil
	}}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), dir, entries)
	if err != nil {
		t.Fatalf("PlanSync: %v", err)
	}
	return plan
}

func stepsOfType[T domain.Step](steps []domain.Step) []T {
	var out []T
	for _, s := range steps {
		if v, ok := s.(T); ok {
			out = append(out, v)
		}
	}
	return out
}
