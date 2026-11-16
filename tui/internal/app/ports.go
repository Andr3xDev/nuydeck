package app

import (
	"context"
	"os/exec"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Inspector holds every read the use cases need; it is always the real
// implementation, also in dry-run.
type Inspector interface {
	// Diff returns the itemized differences needed to make dst equal src for one entry (rsync -n -i --delete).
	Diff(ctx context.Context, e manifest.ResolvedEntry, src, dst domain.Endpoint) ([]domain.Change, error)
	LinkTarget(path string) (string, error)
	PathKind(path string) (domain.NodeKind, error)
	NewestMTime(ctx context.Context, path string, filter manifest.Filter) (time.Time, error)
	// RepoTimes receives the repo root and the entry path relative to it ("<root>/<path>").
	RepoTimes(ctx context.Context, repo, rel string) (domain.RepoTimes, error)
	// ListBackups returns backups newest first.
	ListBackups(ctx context.Context) ([]domain.Backup, error)
	// LoadState never fails; a non-empty warning means defaults were used.
	LoadState(ctx context.Context) (state.State, string)
}

// Runner is the only way anything is modified; dry-run swaps in a recorder.
type Runner interface {
	Run(ctx context.Context, s domain.Step, progress func(domain.StepProgress)) domain.StepResult
}

// Terminal hands the real terminal to a child process, for the one interactive step.
type Terminal interface {
	Run(ctx context.Context, cmd *exec.Cmd) error
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
