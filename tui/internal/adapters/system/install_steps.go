package system

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/sysx"
)

const (
	stagedSuffix = ".nuydeck-new"
	tempPattern  = "nuydeck-write-*"

	removeLongOption   = "--remove"
	minRemoveAbbrevLen = len("--rem")
	privateDirMode     = fs.FileMode(0o700)
	groupWorldWritable = fs.FileMode(0o022)
)

// sudoPrefix keeps sudo non-interactive and ends its option parsing; the TUI's tty holds the timestamp (design D19).
var sudoPrefix = []string{"sudo", "-n", "--"}

var errNoTerminal = errors.New("interactive step needs a terminal")

func (r Runner) exec(ctx context.Context, opts execx.Options, name string, args ...string) (execx.Result, error) {
	if r.Exec != nil {
		return r.Exec(ctx, opts, name, args...)
	}
	return execx.RunWith(ctx, opts, name, args...)
}

func (r Runner) runCommand(ctx context.Context, s plan.CommandStep, progress func(plan.StepProgress)) plan.StepResult {
	if len(s.Argv) == 0 {
		return failure(errors.New("command step has no argv"))
	}
	if err := refuseRemoval(s.Argv); err != nil {
		return failure(err)
	}
	argv := s.Argv
	if s.Sudo {
		argv = append(append([]string(nil), sudoPrefix...), argv...)
	}
	if s.Interactive {
		return r.runInteractive(ctx, s, argv)
	}

	stdout, stderr := plan.NewLineWriter(progress), plan.NewLineWriter(progress)
	res, err := r.exec(ctx, execx.Options{Stdout: stdout, Stderr: stderr, Dir: s.Dir}, argv[0], argv[1:]...)
	stdout.Flush()
	stderr.Flush()
	return commandResult(s.Argv[0], res, err)
}

func (r Runner) runInteractive(ctx context.Context, s plan.CommandStep, argv []string) plan.StepResult {
	if r.Terminal == nil {
		return failure(errNoTerminal)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = s.Dir
	if err := r.Terminal.Run(ctx, cmd); err != nil {
		return failure(fmt.Errorf("%s: %w", s.Argv[0], err))
	}
	return plan.StepResult{Status: plan.StepOK}
}

// refuseRemoval blocks pacman and paru removal operations: the installer never uninstalls.
func refuseRemoval(argv []string) error {
	for i, word := range argv {
		if base := filepath.Base(word); base != "pacman" && base != "paru" {
			continue
		}
		for _, arg := range argv[i+1:] {
			isShortCluster := strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--")
			// pacman accepts any unambiguous long-option prefix; shorter than "--rem" is ambiguous with --refresh.
			isRemoveAbbrev := len(arg) >= minRemoveAbbrevLen && strings.HasPrefix(removeLongOption, arg)
			if isRemoveAbbrev || isShortCluster && strings.Contains(arg, "R") {
				return fmt.Errorf("refusing removal command: %s", strings.Join(argv, " "))
			}
		}
	}
	return nil
}

// stagingDir returns the user-private directory that holds the temp file handed to root.
// A directory others can write to would let them swap the file between the write and `sudo install`.
func (r Runner) stagingDir() (string, error) {
	dir := r.TempDir
	if dir == "" {
		dir = os.Getenv("XDG_RUNTIME_DIR")
	}
	if dir == "" {
		dir = filepath.Join(r.Home, ".cache", "nuydeck", "tmp")
		if err := os.MkdirAll(dir, privateDirMode); err != nil {
			return "", err
		}
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Geteuid() {
		return "", fmt.Errorf("staging dir %s is not owned by the current user", dir)
	}
	if info.Mode().Perm()&groupWorldWritable != 0 {
		return "", fmt.Errorf("staging dir %s is writable by group or others", dir)
	}
	return dir, nil
}

func (r Runner) lstat(path string) (fs.FileInfo, error) {
	if r.Lstat != nil {
		return r.Lstat(path)
	}
	return os.Lstat(path)
}

// writePrivileged backs up the target, stages the content as root beside it and moves it into place.
func (r Runner) writePrivileged(ctx context.Context, s plan.WriteFileStep) plan.StepResult {
	if !sysx.IsAllowedSystemPath(s.Path) {
		return failure(fmt.Errorf("privileged write refused: %s is not allow-listed", s.Path))
	}
	if s.BackupTo != "" {
		if _, err := r.lstat(s.Path); err == nil {
			if err := r.sudo(ctx, "cp", "-p", "--", s.Path, s.BackupTo); err != nil {
				return failure(err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return failure(err)
		}
	}

	dir, err := r.stagingDir()
	if err != nil {
		return failure(err)
	}
	tmp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return failure(err)
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(s.Content)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return failure(err)
	}

	staged := s.Path + stagedSuffix
	mode := fmt.Sprintf("%o", s.Mode.Perm())
	if err := r.sudo(ctx, "install", "-m", mode, "-o", "root", "-g", "root", "--", tmp.Name(), staged); err != nil {
		r.discardStaged(ctx, staged)
		return failure(err)
	}
	if err := r.sudo(ctx, "mv", "-f", "--", staged, s.Path); err != nil {
		r.discardStaged(ctx, staged)
		return failure(err)
	}
	return plan.StepResult{Status: plan.StepOK}
}

// discardStaged is best effort and survives a cancelled ctx, which is a likely cause of the failure.
func (r Runner) discardStaged(ctx context.Context, staged string) {
	_ = r.sudo(context.WithoutCancel(ctx), "rm", "-f", "--", staged)
}

func (r Runner) sudo(ctx context.Context, argv ...string) error {
	full := append(append([]string(nil), sudoPrefix...), argv...)
	res, err := r.exec(ctx, execx.Options{}, full[0], full[1:]...)
	return commandResult(argv[0], res, err).Err
}

func commandResult(name string, res execx.Result, err error) plan.StepResult {
	switch {
	case err != nil:
		return failure(fmt.Errorf("%s: %w", name, err))
	case res.ExitCode != 0:
		return failure(fmt.Errorf("%s exited with %d: %s", name, res.ExitCode, strings.TrimSpace(res.Stderr)))
	}
	return plan.StepResult{Status: plan.StepOK}
}

func failure(err error) plan.StepResult {
	return plan.StepResult{Status: plan.StepFailed, Err: err}
}
