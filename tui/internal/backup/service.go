package backup

import (
	"context"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

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

// DeleteBackup runs a DeleteBackupStep only when confirmed; the bool reports whether a step ran.
func (s *Service) DeleteBackup(ctx context.Context, dir string, confirmed bool) (plan.StepResult, bool) {
	if !confirmed {
		return plan.StepResult{}, false
	}
	return s.Runner.Run(ctx, plan.DeleteBackupStep{Dir: dir}, nil), true
}
