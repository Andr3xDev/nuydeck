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
	applied := map[ItemKey]bool{}

	run := func(step plan.Step) (plan.StepResult, bool) {
		r := s.Runner.Run(ctx, step, progress)
		res.Outcomes = append(res.Outcomes, StepOutcome{Step: step, Result: r})
		if r.Status != plan.StepFailed {
			return r, true
		}
		if res.Err == nil {
			res.Err = r.Err
			if res.Err == nil {
				res.Err = errors.New(cmp.Or(r.Detail, "step failed"))
			}
		}
		return r, false
	}

	func() {
		if files := BackupFiles(items, opts.Hard); opts.Backup && len(files) > 0 {
			if _, ok := run(BackupStep(syncPlan.Direction, files)); !ok {
				return
			}
		}
		for _, g := range GroupByEntry(items) {
			paths, group := CopyPaths(g)
			if len(paths) == 0 {
				continue
			}
			r, ok := run(plan.CopyStep{Entry: g[0].Resolved, Direction: syncPlan.Direction, Src: g[0].Src, Dst: g[0].Dst, Paths: paths, Dir: !g[0].FileEntry})
			for _, it := range group {
				if ok || slices.Contains(r.Applied, it.Change.Path) {
					applied[it.Key()] = true
				}
			}
			if !ok {
				return
			}
		}
		if opts.Hard {
			for _, it := range orderedDeletes(items) {
				if _, ok := run(plan.DeleteStep{Path: it.DstPath(), Root: it.Dst}); !ok {
					return
				}
				applied[it.Key()] = true
			}
		}
		if syncPlan.Direction != plan.Restore {
			if _, ok := run(plan.SaveStateStep{State: s.syncedState(ctx, syncPlan.Direction)}); !ok {
				return
			}
		}
	}()

	for _, it := range items {
		result := ItemNotApplied
		if applied[it.Key()] {
			result = ItemApplied
		}
		res.Items = append(res.Items, ItemResult{Key: it.Key(), Result: result})
	}
	run(plan.AppendLogStep{Entry: s.logEntry(syncPlan.Direction, opts.DryRun, items, res)})
	return res
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
