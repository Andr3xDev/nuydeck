package app

import (
	"context"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
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
