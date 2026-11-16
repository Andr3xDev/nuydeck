package app

import (
	"context"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Inspector holds every read the use cases need; it is always the real
// implementation, also in dry-run.
type Inspector interface {
	cfgsync.Inspector
	// ListBackups returns backups newest first.
	ListBackups(ctx context.Context) ([]backup.Backup, error)
	// LoadState never fails; a non-empty warning means defaults were used.
	LoadState(ctx context.Context) (state.State, string)
}

// SystemProbe holds the read-only machine queries of the install and the package screens; always the real implementation.
type SystemProbe interface {
	DetectGPUs(ctx context.Context) ([]domain.PCIDevice, error)
	InstalledPackages(ctx context.Context) (map[string]bool, error)
	HasCommand(name string) bool
	LoginShell(ctx context.Context) (string, error)
	// ReadSystemFile reads an allow-listed system file; a missing file returns an error wrapping fs.ErrNotExist.
	ReadSystemFile(path string) ([]byte, error)
	IsRoot() bool
}
