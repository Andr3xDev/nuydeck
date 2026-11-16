package osexec

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"

	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/rsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Runner is the real plan.Runner.
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
