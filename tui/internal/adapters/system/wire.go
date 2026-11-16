package system

import (
	"context"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/gitx"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/rsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Inspector is the real app.Inspector.
type Inspector struct {
	Home string
}

func (i Inspector) Diff(ctx context.Context, e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error) {
	kind, err := osfs.PathKind(src.Path)
	if err != nil {
		return nil, err
	}
	res, err := rsync.Plan(ctx, rsync.Request{Src: src.Path, Dst: dst.Path, Dir: kind == plan.KindDir, Filter: e.Filter})
	if err != nil {
		return nil, err
	}
	changes := make([]plan.Change, len(res.Items))
	for n, it := range res.Items {
		changes[n] = plan.Change{Entry: e.Entry, Path: it.Path, Type: it.Type, Kind: it.Kind}
	}
	return changes, nil
}

func (Inspector) LinkTarget(path string) (string, error)      { return osfs.LinkTarget(path) }
func (Inspector) PathKind(path string) (plan.NodeKind, error) { return osfs.PathKind(path) }

func (Inspector) NewestMTime(_ context.Context, path string, filter manifest.Filter) (time.Time, error) {
	return osfs.NewestMTime(path, filter)
}

func (Inspector) RepoTimes(ctx context.Context, repo, rel string) (plan.RepoTimes, error) {
	return gitx.Reader{}.RepoTimes(ctx, repo, rel)
}

func (i Inspector) ListBackups(ctx context.Context) ([]backup.Backup, error) {
	return backup.ListBackups(ctx, i.Home)
}

func (i Inspector) LoadState(context.Context) (state.State, string) {
	return state.LoadState(state.StatePath(i.Home))
}
