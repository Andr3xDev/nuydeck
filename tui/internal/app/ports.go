package app

import (
	"context"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Inspector holds every read the use cases need; it is always the real
// implementation, also in dry-run.
type Inspector interface {
	// Diff returns the itemized differences needed to make dst equal src for one entry (rsync -n -i --delete).
	Diff(ctx context.Context, e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error)
	LinkTarget(path string) (string, error)
	PathKind(path string) (plan.NodeKind, error)
	NewestMTime(ctx context.Context, path string, filter manifest.Filter) (time.Time, error)
	// RepoTimes receives the repo root and the entry path relative to it ("<root>/<path>").
	RepoTimes(ctx context.Context, repo, rel string) (plan.RepoTimes, error)
	// ListBackups returns backups newest first.
	ListBackups(ctx context.Context) ([]backup.Backup, error)
	// LoadState never fails; a non-empty warning means defaults were used.
	LoadState(ctx context.Context) (state.State, string)
}

// SystemProbe holds the read-only machine queries of the install and the package screens; always the real implementation.
type SystemProbe interface {
	DetectGPUs(ctx context.Context) ([]domain.PCIDevice, error)
	InstalledPackages(ctx context.Context) (map[string]bool, error)
	ExplicitPackages(ctx context.Context) ([]string, error)
	HasCommand(name string) bool
	LoginShell(ctx context.Context) (string, error)
	// ReadSystemFile reads an allow-listed system file; a missing file returns an error wrapping fs.ErrNotExist.
	ReadSystemFile(path string) ([]byte, error)
	IsRoot() bool
}
