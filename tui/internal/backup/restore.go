package backup

import (
	"context"
	"path"
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// PlanRestore plans a restore from the backup; roots come from the backup metadata and deletions are never offered.
func (s *Service) PlanRestore(ctx context.Context, b Backup) (cfgsync.SyncPlan, error) {
	roots := make([]manifest.Root, 0, len(b.Roots))
	for root := range b.Roots {
		roots = append(roots, root)
	}
	slices.Sort(roots)
	syncPlan := cfgsync.SyncPlan{Direction: plan.Restore, Selection: cfgsync.Selection{}}
	for _, root := range roots {
		src, dst := path.Join(b.Dir, string(root)), b.Roots[root]
		e := manifest.ResolvedEntry{Entry: manifest.Entry{Root: root}, RepoPath: src, DestPath: dst}
		items, err := s.PlanEntry(ctx, plan.Restore, e, src, dst, nil)
		if err != nil {
			return cfgsync.SyncPlan{}, err
		}
		for _, it := range items {
			if it.Change.Type == plan.Delete {
				continue
			}
			syncPlan.Items = append(syncPlan.Items, it)
			syncPlan.Selection[it.Key()] = !it.Blocked
		}
	}
	return syncPlan, nil
}
