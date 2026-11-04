package app

import (
	"context"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// Inspector holds every read the use cases need; it is always the real
// implementation, also in dry-run.
type Inspector interface {
	// Diff returns the itemized differences needed to make dst equal src for one entry (rsync -n -i --delete).
	Diff(ctx context.Context, e domain.ResolvedEntry, src, dst domain.Endpoint) ([]domain.Change, error)
	LinkTarget(path string) (string, error)
	PathKind(path string) (domain.NodeKind, error)
	NewestMTime(ctx context.Context, path string, filter domain.Filter) (time.Time, error)
	// RepoTimes receives the repo root and the entry path relative to it ("<root>/<path>").
	RepoTimes(ctx context.Context, repo, rel string) (domain.RepoTimes, error)
	// ListBackups returns backups newest first.
	ListBackups(ctx context.Context) ([]domain.Backup, error)
	// LoadState never fails; a non-empty warning means defaults were used.
	LoadState(ctx context.Context) (domain.State, string)
}

// Runner is the only way anything is modified; dry-run swaps in a recorder.
type Runner interface {
	Run(ctx context.Context, s domain.Step, progress func(domain.StepProgress)) domain.StepResult
}
