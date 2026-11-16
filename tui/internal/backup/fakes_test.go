package backup

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const testRepo = "/repo"

var testNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

type diffCall struct{ src, dst string }

type fakeInspector struct {
	mu        sync.Mutex
	diff      func(e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error)
	diffCalls []diffCall
}

func (f *fakeInspector) Diff(_ context.Context, e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error) {
	f.mu.Lock()
	f.diffCalls = append(f.diffCalls, diffCall{src.Path, dst.Path})
	f.mu.Unlock()
	return f.diff(e, src, dst)
}

func (f *fakeInspector) LinkTarget(string) (string, error) { return "", nil }

func (f *fakeInspector) PathKind(string) (plan.NodeKind, error) { return plan.KindDir, nil }

func (f *fakeInspector) NewestMTime(context.Context, string, manifest.Filter) (time.Time, error) {
	return time.Time{}, nil
}

func (f *fakeInspector) RepoTimes(context.Context, string, string) (plan.RepoTimes, error) {
	return plan.RepoTimes{}, nil
}

func (f *fakeInspector) LoadState(context.Context) (state.State, string) {
	return state.State{}, ""
}

type fakeRunner struct {
	mu    sync.Mutex
	steps []plan.Step
}

func (r *fakeRunner) Run(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
	r.mu.Lock()
	r.steps = append(r.steps, s)
	r.mu.Unlock()
	return plan.StepResult{Status: plan.StepOK}
}

func newService(t *testing.T, in *fakeInspector, r plan.Runner) *Service {
	t.Helper()
	return &Service{
		Service: &cfgsync.Service{Inspector: in, LoadState: in.LoadState, Runner: r, RepoDir: testRepo, Now: func() time.Time { return testNow }},
	}
}

func diffReturning(changes ...plan.Change) func(manifest.ResolvedEntry, plan.Endpoint, plan.Endpoint) ([]plan.Change, error) {
	return func(manifest.ResolvedEntry, plan.Endpoint, plan.Endpoint) ([]plan.Change, error) {
		return changes, nil
	}
}

func change(p string, t plan.ChangeType, k plan.NodeKind) plan.Change {
	return plan.Change{Path: p, Type: t, Kind: k}
}

func selectAll(syncPlan cfgsync.SyncPlan) cfgsync.Selection {
	sel := cfgsync.Selection{}
	for _, it := range syncPlan.Items {
		sel[it.Key()] = true
	}
	return sel
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
