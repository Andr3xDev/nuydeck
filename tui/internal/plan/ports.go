package plan

import (
	"context"
	"os/exec"
)

// Runner is the only way anything is modified; dry-run swaps in a recorder.
type Runner interface {
	Run(ctx context.Context, s Step, progress func(StepProgress)) StepResult
}

// Terminal hands the real terminal to a child process, for the one interactive step.
type Terminal interface {
	Run(ctx context.Context, cmd *exec.Cmd) error
}
