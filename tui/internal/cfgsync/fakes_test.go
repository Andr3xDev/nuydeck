package cfgsync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
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
	diff        func(e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error)
	diffCalls   []diffCall
	linkTargets map[string]string
	kinds       map[string]plan.NodeKind
	mtimes      map[string]time.Time
	repoTimes   map[string]plan.RepoTimes
	state       state.State
	stateWarn   string
}

func (f *fakeInspector) Diff(_ context.Context, e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error) {
	f.mu.Lock()
	f.diffCalls = append(f.diffCalls, diffCall{src.Path, dst.Path})
	f.mu.Unlock()
	return f.diff(e, src, dst)
}

func (f *fakeInspector) LinkTarget(p string) (string, error) { return f.linkTargets[p], nil }

func (f *fakeInspector) PathKind(p string) (plan.NodeKind, error) {
	if k, ok := f.kinds[p]; ok {
		return k, nil
	}
	return plan.KindDir, nil
}

func (f *fakeInspector) NewestMTime(_ context.Context, p string, _ manifest.Filter) (time.Time, error) {
	return f.mtimes[p], nil
}

func (f *fakeInspector) RepoTimes(_ context.Context, _, rel string) (plan.RepoTimes, error) {
	return f.repoTimes[rel], nil
}

func (f *fakeInspector) LoadState(context.Context) (state.State, string) {
	return f.state, f.stateWarn
}

type fakeRunner struct {
	mu     sync.Mutex
	steps  []plan.Step
	result func(plan.Step) plan.StepResult
}

func (r *fakeRunner) Run(_ context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
	r.mu.Lock()
	r.steps = append(r.steps, s)
	r.mu.Unlock()
	if progress != nil {
		progress(plan.StepProgress{Path: "tick"})
	}
	if r.result != nil {
		return r.result(s)
	}
	return plan.StepResult{Status: plan.StepOK}
}

func resolved(root manifest.Root, p string, excludes ...string) manifest.ResolvedEntry {
	return manifest.ResolvedEntry{
		Entry:    manifest.Entry{Root: root, Path: p},
		RepoPath: testRepo + "/" + string(root) + "/" + p,
		DestPath: rootDirs[root] + "/" + p,
		Filter:   manifest.Filter{Excludes: excludes},
	}
}

func newService(t *testing.T, in *fakeInspector, r plan.Runner) *Service {
	t.Helper()
	return &Service{Inspector: in, LoadState: in.LoadState, Runner: r, RepoDir: testRepo, Now: func() time.Time { return testNow }}
}

func diffReturning(changes ...plan.Change) func(manifest.ResolvedEntry, plan.Endpoint, plan.Endpoint) ([]plan.Change, error) {
	return func(manifest.ResolvedEntry, plan.Endpoint, plan.Endpoint) ([]plan.Change, error) {
		return changes, nil
	}
}

func change(p string, t plan.ChangeType, k plan.NodeKind) plan.Change {
	return plan.Change{Path: p, Type: t, Kind: k}
}

func selectAll(plan SyncPlan) Selection {
	sel := Selection{}
	for _, it := range plan.Items {
		sel[it.Key()] = true
	}
	return sel
}

func planOf(t *testing.T, dir plan.Direction, entries []manifest.ResolvedEntry, byEntry map[string][]plan.Change) SyncPlan {
	t.Helper()
	in := &fakeInspector{diff: func(e manifest.ResolvedEntry, _, _ plan.Endpoint) ([]plan.Change, error) {
		return byEntry[e.Path], nil
	}}
	plan, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), dir, entries)
	if err != nil {
		t.Fatalf("PlanSync: %v", err)
	}
	return plan
}

func stepsOfType[T plan.Step](steps []plan.Step) []T {
	var out []T
	for _, s := range steps {
		if v, ok := s.(T); ok {
			out = append(out, v)
		}
	}
	return out
}
