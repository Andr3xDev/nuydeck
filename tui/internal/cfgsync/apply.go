package cfgsync

import (
	"cmp"
	"context"
	"errors"
	"path/filepath"
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
	batches := Steps(syncPlan, sel, syncPlan.Resolutions, opts)
	if syncPlan.Direction != plan.Restore && !syncPlan.Partial {
		if st, warning := s.syncedState(ctx, syncPlan.Direction); warning == "" {
			batches = append(batches, Batch{Step: plan.SaveStateStep{State: st}})
		} else {
			res.Notices = append(res.Notices, "last sync not recorded: state.toml could not be read")
		}
	}
	steps := make([]plan.Item, len(batches))
	for i, b := range batches {
		steps[i] = plan.Item{Step: b.Step}
	}
	owners := func(i int) []PlanItem { return batches[i].Items }

	// The recorder keeps every outcome, failed ones too, and hands progress straight to the caller.
	recorder := runnerFunc(func(ctx context.Context, step plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		r := s.Runner.Run(ctx, step, progress)
		res.Outcomes = append(res.Outcomes, StepOutcome{Step: step, Result: r})
		return r
	})
	res.Err = plan.Apply(ctx, recorder, steps, plan.Hooks{}, func(plan.Event) {}).Err

	applied := map[ItemKey]bool{}
	for i, o := range res.Outcomes {
		for _, it := range owners(i) {
			_, isCopy := o.Step.(plan.CopyStep)
			if o.Result.Detail == plan.KeptNotEmpty {
				continue
			}
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

// Batch is one step with the plan items it applies.
type Batch struct {
	Step  plan.Step
	Items []PlanItem
}

// Steps lists the steps that apply the selection, without the final state step.
// Items with a resolution are written from it instead of copied.
func Steps(p SyncPlan, sel Selection, res Resolutions, opts ApplyOptions) []Batch {
	items := SelectedItems(p, sel)
	var batches []Batch
	var edited, plain []PlanItem
	for _, it := range items {
		if _, ok := res[it.Key()]; ok {
			edited = append(edited, it)
		} else {
			plain = append(plain, it)
		}
	}

	if files := BackupFiles(items, opts.Hard); (opts.Backup || opts.Mandatory) && len(files) > 0 {
		batches = append(batches, Batch{Step: BackupStep(p.Direction, files)})
	}
	for _, g := range GroupByEntry(plain) {
		paths, group := CopyPaths(g)
		if len(paths) == 0 {
			continue
		}
		batches = append(batches, Batch{Step: plan.CopyStep{Entry: g[0].Resolved, Direction: p.Direction, Src: g[0].Src, Dst: g[0].Dst, Paths: paths, Dir: !g[0].FileEntry}, Items: group})
	}
	for _, it := range edited {
		e := res[it.Key()]
		batches = append(batches, Batch{Step: plan.WriteFileStep{Path: it.DstPath(), Content: e.Content, Mode: e.Mode, Root: writeRoot(it)}, Items: []PlanItem{it}})
	}
	if opts.Hard {
		for _, it := range orderedDeletes(plain) {
			batches = append(batches, Batch{Step: plan.DeleteStep{Path: it.DstPath(), Root: it.Dst}, Items: []PlanItem{it}})
		}
	}
	return batches
}

// writeRoot is the root an edited item may be written under: a file entry's own path is the
// destination, so its parent bounds the write.
func writeRoot(it PlanItem) string {
	if it.FileEntry {
		return filepath.Dir(it.Dst)
	}
	return it.Dst
}

type runnerFunc func(ctx context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult

func (f runnerFunc) Run(ctx context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
	return f(ctx, s, progress)
}

// syncedState returns the state to save and the load warning; with a warning the file is unreadable
// and saving would overwrite it.
func (s *Service) syncedState(ctx context.Context, dir plan.Direction) (state.State, string) {
	// Re-read so ignores and state saved since planning are not overwritten.
	current, warning := s.LoadState(ctx)
	current.RepoPath = s.RepoDir
	current.LastSync = &state.LastSync{Time: s.Now(), Direction: directionName(dir)}
	return current, warning
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
