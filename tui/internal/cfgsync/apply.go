package cfgsync

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

func (s *Service) Apply(ctx context.Context, syncPlan SyncPlan, sel Selection, opts ApplyOptions, progress func(plan.StepProgress)) ApplyResult {
	items := SelectedItems(syncPlan, sel)
	if len(items) == 0 {
		return ApplyResult{Nothing: true}
	}
	var res ApplyResult
	var steps []plan.Item
	// owners[i] are the plan items that steps[i] applies.
	var owners [][]PlanItem
	add := func(step plan.Step, owned ...PlanItem) {
		steps = append(steps, plan.Item{Step: step})
		owners = append(owners, owned)
	}

	if files := BackupFiles(items, opts.Hard); opts.Backup && len(files) > 0 {
		add(BackupStep(syncPlan.Direction, files))
	}
	for _, g := range GroupByEntry(items) {
		paths, group := CopyPaths(g)
		if len(paths) == 0 {
			continue
		}
		add(plan.CopyStep{Entry: g[0].Resolved, Direction: syncPlan.Direction, Src: g[0].Src, Dst: g[0].Dst, Paths: paths, Dir: !g[0].FileEntry}, group...)
	}
	if opts.Hard {
		for _, it := range orderedDeletes(items) {
			add(plan.DeleteStep{Path: it.DstPath(), Root: it.Dst}, it)
		}
	}
	if syncPlan.Direction != plan.Restore {
		add(plan.SaveStateStep{State: s.syncedState(ctx, syncPlan.Direction)})
	}

	// The recorder keeps every outcome, failed ones too, and hands progress straight to the caller.
	recorder := runnerFunc(func(ctx context.Context, step plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		r := s.Runner.Run(ctx, step, progress)
		res.Outcomes = append(res.Outcomes, StepOutcome{Step: step, Result: r})
		return r
	})
	res.Err = plan.Apply(ctx, recorder, steps, plan.Hooks{}, func(plan.Event) {}).Err

	applied := map[ItemKey]bool{}
	for i, o := range res.Outcomes {
		for _, it := range owners[i] {
			_, isCopy := o.Step.(plan.CopyStep)
			if o.Result.Status != plan.StepFailed || isCopy && slices.Contains(o.Result.Applied, it.Change.Path) {
				applied[it.Key()] = true
			}
		}
	}

	for _, it := range items {
		result := ItemNotApplied
		if applied[it.Key()] {
			result = ItemApplied
		}
		res.Items = append(res.Items, ItemResult{Key: it.Key(), Result: result})
	}
	logStep := plan.AppendLogStep{Entry: s.logEntry(syncPlan.Direction, opts.DryRun, items, res)}
	r := s.Runner.Run(ctx, logStep, progress)
	res.Outcomes = append(res.Outcomes, StepOutcome{Step: logStep, Result: r})
	if r.Status == plan.StepFailed && res.Err == nil {
		res.Err = cmp.Or(r.Err, errors.New(cmp.Or(r.Detail, "step failed")))
	}
	return res
}

type runnerFunc func(ctx context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult

func (f runnerFunc) Run(ctx context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
	return f(ctx, s, progress)
}

func (s *Service) syncedState(ctx context.Context, dir plan.Direction) state.State {
	// Re-read so ignores and state saved since planning are not overwritten.
	current, _ := s.LoadState(ctx)
	current.RepoPath = s.RepoDir
	current.LastSync = &state.LastSync{Time: s.Now(), Direction: directionName(dir)}
	return current
}

func (s *Service) logEntry(dir plan.Direction, dryRun bool, items []PlanItem, res ApplyResult) state.LogEntry {
	entry := state.LogEntry{
		Time:      s.Now().Format(time.RFC3339),
		Op:        "sync",
		Direction: directionName(dir),
		DryRun:    dryRun,
		Result:    "ok",
	}
	if dir == plan.Restore {
		entry.Op = "restore"
	}
	if res.Err != nil {
		entry.Result, entry.Error = "failed", res.Err.Error()
	}
	for i, it := range items {
		entry.Items = append(entry.Items, state.LogItem{Path: it.rel(), Kind: kindName(it.Change.Kind), Result: res.Items[i].Result})
	}
	return entry
}
