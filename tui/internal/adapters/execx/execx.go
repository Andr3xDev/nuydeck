package execx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

// Result is the outcome of one finished command.
type Result struct {
	ExitCode int
	Stderr   string
}

// Run executes name with args, streaming stdout to out (may be nil). A
// non-zero exit is reported in Result with a nil error; err is set only when
// the command could not run or was cancelled.
func Run(ctx context.Context, out io.Writer, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Fixed locale keeps rsync output parseable.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stdout = out
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := Result{Stderr: stderr.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && ctx.Err() == nil {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, err
}
