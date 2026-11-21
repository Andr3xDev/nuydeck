package backup

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"strings"

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

// PlanRestore plans a restore from the backup; deletions are never offered.
func (s *Service) PlanRestore(ctx context.Context, b Backup) (cfgsync.SyncPlan, error) {
	if err := CheckRestorable(b); err != nil {
		return cfgsync.SyncPlan{}, err
	}
	syncPlan := cfgsync.SyncPlan{Direction: plan.Restore, Selection: cfgsync.Selection{}}
	// Diff only the recorded files: diffing a whole root (home) walks unreadable dirs and fails.
	// meta.toml is untrusted: rel must stay inside the root and the root must be absolute.
	for _, file := range b.Files {
		rootName, rel, _ := strings.Cut(file, "/")
		root := manifest.Root(rootName)
		destRoot, ok := b.Roots[root]
		if !ok || !filepath.IsAbs(destRoot) {
			syncPlan.Notices = append(syncPlan.Notices, "skipped: no recorded root: "+file)
			continue
		}
		if !filepath.IsLocal(rel) || path.Clean(rel) != rel {
			syncPlan.Notices = append(syncPlan.Notices, "skipped: invalid path in backup: "+file)
			continue
		}
		src, dst := path.Join(b.Dir, rootName, rel), path.Join(destRoot, rel)
		e := manifest.ResolvedEntry{Entry: manifest.Entry{Root: root, Path: rel}, RepoPath: src, DestPath: dst}
		items, err := s.PlanEntry(ctx, plan.Restore, e, src, dst, nil)
		if errors.Is(err, cfgsync.ErrSourceMissing) {
			syncPlan.Notices = append(syncPlan.Notices, "skipped: missing in backup: "+file)
			continue
		}
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
