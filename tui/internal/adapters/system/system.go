// Package system composes the concrete adapters into the app ports.
package system

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/fsx"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/gitx"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/rsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/tomlx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// Inspector is the real app.Inspector.
type Inspector struct {
	Home string
}

func (i Inspector) Diff(ctx context.Context, e domain.ResolvedEntry, src, dst domain.Endpoint) ([]domain.Change, error) {
	kind, err := fsx.PathKind(src.Path)
	if err != nil {
		return nil, err
	}
	res, err := rsync.Plan(ctx, rsync.Request{Src: src.Path, Dst: dst.Path, Dir: kind == domain.KindDir, Filter: e.Filter})
	if err != nil {
		return nil, err
	}
	changes := make([]domain.Change, len(res.Items))
	for n, it := range res.Items {
		changes[n] = domain.Change{Entry: e.Entry, Path: it.Path, Type: it.Type, Kind: it.Kind}
	}
	return changes, nil
}

func (Inspector) LinkTarget(path string) (string, error)        { return fsx.LinkTarget(path) }
func (Inspector) PathKind(path string) (domain.NodeKind, error) { return fsx.PathKind(path) }

func (Inspector) NewestMTime(_ context.Context, path string, filter domain.Filter) (time.Time, error) {
	return fsx.NewestMTime(path, filter)
}

func (Inspector) RepoTimes(ctx context.Context, repo, rel string) (domain.RepoTimes, error) {
	return gitx.Reader{}.RepoTimes(ctx, repo, rel)
}

func (i Inspector) ListBackups(ctx context.Context) ([]domain.Backup, error) {
	return fsx.ListBackups(ctx, i.Home)
}

func (i Inspector) LoadState(context.Context) (domain.State, string) {
	return tomlx.LoadState(tomlx.StatePath(i.Home))
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

func (r Runner) Run(ctx context.Context, s domain.Step, progress func(domain.StepProgress)) domain.StepResult {
	switch s := s.(type) {
	case domain.CopyStep:
		req := rsync.Request{Src: s.Src, Dst: s.Dst, Dir: s.Dir, Filter: s.Entry.Filter}
		var onItem func(string)
		if progress != nil {
			onItem = func(p string) { progress(domain.StepProgress{Path: p}) }
		}
		if err := rsync.Apply(ctx, req, s.Paths, onItem); err != nil {
			return domain.StepResult{Status: domain.StepFailed, Err: err}
		}
		return domain.StepResult{Status: domain.StepOK, Applied: s.Paths}
	case domain.SaveStateStep:
		return done(tomlx.SaveState(tomlx.StatePath(r.Home), s.State))
	case domain.AppendLogStep:
		return done(tomlx.AppendLog(tomlx.LogsDir(r.ConfigDir), s.Entry))
	case domain.DeleteStep, domain.BackupStep, domain.DeleteBackupStep, domain.SymlinkStep, domain.WriteSecretsStep:
		return fsx.Runner{Home: r.Home}.Run(ctx, s, progress)
	case domain.WriteFileStep:
		if s.Privileged {
			return r.writePrivileged(ctx, s)
		}
		return fsx.Runner{Home: r.Home}.Run(ctx, s, progress)
	case domain.AppendPackageStep:
		return done(tomlx.AppendPackage(s.Path, s.Entry))
	case domain.CommandStep:
		return r.runCommand(ctx, s, progress)
	}
	return domain.StepResult{Status: domain.StepFailed, Err: fmt.Errorf("unsupported step %T", s)}
}

func done(err error) domain.StepResult {
	if err != nil {
		return domain.StepResult{Status: domain.StepFailed, Err: err}
	}
	return domain.StepResult{Status: domain.StepOK}
}
