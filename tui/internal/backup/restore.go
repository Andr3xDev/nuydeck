package backup

import (
	"context"
	"errors"
	"path"
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

var (
	ErrIncomplete = errors.New("backup is incomplete")
	ErrUnreadable = errors.New("backup is unreadable")
)

// CheckRestorable refuses backups that cannot be restored.
func CheckRestorable(b Backup) error {
	switch {
	case b.Unreadable:
		return ErrUnreadable
	case b.Incomplete:
		return ErrIncomplete
	}
	return nil
}

// PlanRestore plans a restore from the backup; roots come from the backup metadata and deletions are never offered.
func (s *Service) PlanRestore(ctx context.Context, b Backup) (cfgsync.SyncPlan, error) {
	if err := CheckRestorable(b); err != nil {
		return cfgsync.SyncPlan{}, err
	}
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
