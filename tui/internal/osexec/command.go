package osexec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	removeLongOption   = "--remove"
	minRemoveAbbrevLen = len("--rem")
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

	stdout, stderr := newLineWriter(progress), newLineWriter(progress)
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
