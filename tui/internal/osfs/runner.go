package osfs

import (
	"context"
	"fmt"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Runner executes DeleteStep, BackupStep and DeleteBackupStep. Backups live in
// <Home>/.nuydeck-backups. Now defaults to time.Now when nil.
type Runner struct {
	Home string
	Now  func() time.Time
}

func failed(err error) plan.StepResult {
	return plan.StepResult{Status: plan.StepFailed, Err: err}
}

func (r Runner) Run(ctx context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
	switch s := s.(type) {
	case plan.DeleteStep:
		return deletePath(s)
	case plan.BackupStep:
		return r.backup(ctx, s)
	case plan.DeleteBackupStep:
		return r.deleteBackup(s)
	case plan.WriteFileStep:
		return writeFile(s)
	case plan.SymlinkStep:
		return symlink(s)
	case plan.WriteSecretsStep:
		return writeSecrets(s)
	}
	return failed(fmt.Errorf("fsx cannot run %T", s))
}

func (r Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}
