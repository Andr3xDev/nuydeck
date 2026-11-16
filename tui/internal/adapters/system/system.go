// Package system composes the concrete adapters into the app ports.
package system

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
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

// Runner is the real app.Runner.
type Runner struct {
	Home      string
	ConfigDir string
	// Exec runs external commands; nil means execx.RunWith.
	Exec func(ctx context.Context, opts execx.Options, name string, args ...string) (execx.Result, error)
	// Terminal runs the interactive handoff; nil means no terminal.
	Terminal interface {
		Run(ctx context.Context, cmd *exec.Cmd) error
	}
	// TempDir holds the user-owned temp file of privileged writes; it must be owned by the user and not group/world-writable.
	// Empty means $XDG_RUNTIME_DIR, else <Home>/.cache/nuydeck/tmp (created 0700).
	TempDir string
	// Lstat inspects the backup source; nil means os.Lstat.
	Lstat func(path string) (fs.FileInfo, error)
}

func (r Runner) Run(ctx context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
	switch s := s.(type) {
	case plan.CopyStep:
		req := rsync.Request{Src: s.Src, Dst: s.Dst, Dir: s.Dir, Filter: s.Entry.Filter}
		var onItem func(string)
		if progress != nil {
			onItem = func(p string) { progress(plan.StepProgress{Path: p}) }
		}
		if err := rsync.Apply(ctx, req, s.Paths, onItem); err != nil {
			return plan.StepResult{Status: plan.StepFailed, Err: err}
		}
		return plan.StepResult{Status: plan.StepOK, Applied: s.Paths}
	case plan.SaveStateStep:
		return done(state.SaveState(state.StatePath(r.Home), s.State))
	case plan.AppendLogStep:
		return done(state.AppendLog(state.LogsDir(r.ConfigDir), s.Entry))
	case plan.DeleteStep, plan.BackupStep, plan.DeleteBackupStep, plan.SymlinkStep, plan.WriteSecretsStep:
		return osfs.Runner{Home: r.Home}.Run(ctx, s, progress)
	case plan.WriteFileStep:
		if s.Privileged {
			return r.writePrivileged(ctx, s)
		}
		return osfs.Runner{Home: r.Home}.Run(ctx, s, progress)
	case plan.AppendPackageStep:
		return done(manifest.AppendPackage(s.Path, s.Entry))
	case plan.CommandStep:
		return r.runCommand(ctx, s, progress)
	}
	return plan.StepResult{Status: plan.StepFailed, Err: fmt.Errorf("unsupported step %T", s)}
}

func done(err error) plan.StepResult {
	if err != nil {
		return plan.StepResult{Status: plan.StepFailed, Err: err}
	}
	return plan.StepResult{Status: plan.StepOK}
}
