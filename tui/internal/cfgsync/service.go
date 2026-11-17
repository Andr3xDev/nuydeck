package cfgsync

import (
	"context"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Inspector holds every read the sync needs; it is always the real
// implementation, also in dry-run.
type Inspector interface {
	// Diff returns the itemized differences needed to make dst equal src for one entry (rsync -n -i --delete).
	Diff(ctx context.Context, e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error)
	LinkTarget(path string) (string, error)
	PathKind(path string) (plan.NodeKind, error)
	// ReadSide reads one side of an item for the resolver; a missing path is Exists=false, not an error.
	ReadSide(path string) (Side, error)
	NewestMTime(ctx context.Context, path string, filter manifest.Filter) (time.Time, error)
	// RepoTimes receives the repo root and the entry path relative to it ("<root>/<path>").
	RepoTimes(ctx context.Context, repo, rel string) (plan.RepoTimes, error)
}

// Service bundles the use cases over the ports.
type Service struct {
	Inspector Inspector
	// LoadState never fails; a non-empty warning means defaults were used.
	LoadState func(ctx context.Context) (state.State, string)
	Runner    plan.Runner
	RepoDir   string
	Now       func() time.Time
}
