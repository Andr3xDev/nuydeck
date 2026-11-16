package execx

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
)

// Options tunes RunWith. Zero values mean no streaming and the caller's cwd.
type Options struct {
	// Stdout and Stderr receive the child's output live; either may be nil.
	Stdout, Stderr io.Writer
	Dir            string
}

// stderrTailCap bounds Result.Stderr; errors surface at the end of the output.
const stderrTailCap = 16 * 1024

// tailWriter keeps only the last stderrTailCap bytes written to it.
type tailWriter struct{ buf []byte }

func (w *tailWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if extra := len(w.buf) - stderrTailCap; extra > 0 {
		w.buf = append([]byte(nil), w.buf[extra:]...)
	}
	return len(p), nil
}

// RunWith is Run with live stderr and a working directory. The child stays in
// the caller's session: sudo's tty timestamp is per terminal (design D19).
func RunWith(ctx context.Context, opts Options, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Fixed locale keeps rsync output parseable.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	// SIGTERM lets rsync remove its temp files; WaitDelay still escalates to kill.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = cancelWaitDelay
	cmd.Dir = opts.Dir
	var stderr tailWriter
	cmd.Stdout = opts.Stdout
	cmd.Stderr = &stderr
	if opts.Stderr != nil {
		cmd.Stderr = io.MultiWriter(&stderr, opts.Stderr)
	}

	err := cmd.Run()
	res := Result{Stderr: string(stderr.buf)}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && ctx.Err() == nil {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, err
}
