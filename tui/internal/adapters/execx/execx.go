package execx

import (
	"context"
	"io"
	"time"
)

// cancelWaitDelay bounds how long Run waits for I/O after cancel, so an
// orphaned child holding the stderr pipe cannot block the return.
const cancelWaitDelay = 2 * time.Second

// Result is the outcome of one finished command.
type Result struct {
	ExitCode int
	Stderr   string
}

// Run executes name with args, streaming stdout to out (may be nil). A
// non-zero exit is reported in Result with a nil error; err is set only when
// the command could not run or was cancelled.
func Run(ctx context.Context, out io.Writer, name string, args ...string) (Result, error) {
	return RunWith(ctx, Options{Stdout: out}, name, args...)
}
