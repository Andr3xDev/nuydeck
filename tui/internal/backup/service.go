package backup

import (
	"context"
	"errors"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// ErrNothingToBackUp means no manifest entry exists on the PC.
var ErrNothingToBackUp = errors.New("nothing to back up")

const manualFiltersNotice = "Manifest filters are not applied, excluded files are included."

// Lister returns the existing backups newest first.
type Lister interface {
	ListBackups(ctx context.Context) ([]Backup, error)
}

// Service holds the backup use cases; the embedded sync service plans and runs restores.
type Service struct {
	*cfgsync.Service
	Lister Lister
}

// ListBackups returns the existing backups newest first.
func (s *Service) ListBackups(ctx context.Context) ([]Backup, error) {
	return s.Lister.ListBackups(ctx)
}

// PlanManual snapshots the destination of every entry that exists on the PC.
func (s *Service) PlanManual(_ context.Context, entries []manifest.ResolvedEntry) (plan.Plan, error) {
	var items []cfgsync.PlanItem
	for _, e := range entries {
		kind, err := s.Inspector.PathKind(e.DestPath)
		if err != nil {
			return plan.Plan{}, err
		}
		if kind == plan.KindNone {
			continue
		}
		items = append(items, cfgsync.PlanItem{
			Change:    plan.Change{Entry: e.Entry, Kind: kind},
			Resolved:  e,
			Dst:       e.DestPath,
			FileEntry: true,
		})
	}
	if len(items) == 0 {
		return plan.Plan{}, ErrNothingToBackUp
	}
	return plan.Plan{
		Steps:   []plan.Step{cfgsync.BackupStep(plan.Manual, items)},
		Notices: []string{manualFiltersNotice},
	}, nil
}

// DeleteBackup runs a DeleteBackupStep only when confirmed; the bool reports whether a step ran.
func (s *Service) DeleteBackup(ctx context.Context, dir string, confirmed bool) (plan.StepResult, bool) {
	if !confirmed {
		return plan.StepResult{}, false
	}
	return s.Runner.Run(ctx, plan.DeleteBackupStep{Dir: dir}, nil), true
}
