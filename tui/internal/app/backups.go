package app

import (
	"context"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// ListBackups returns the existing backups newest first.
func (s *Service) ListBackups(ctx context.Context) ([]backup.Backup, error) {
	return s.Inspector.ListBackups(ctx)
}

// DeleteBackup runs a DeleteBackupStep only when confirmed; the bool reports whether a step ran.
func (s *Service) DeleteBackup(ctx context.Context, dir string, confirmed bool) (plan.StepResult, bool) {
	if !confirmed {
		return plan.StepResult{}, false
	}
	return s.Runner.Run(ctx, plan.DeleteBackupStep{Dir: dir}, nil), true
}
