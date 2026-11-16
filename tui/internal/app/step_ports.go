package app

import (
	"context"
	"os/exec"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// Runner is the only way anything is modified; dry-run swaps in a recorder.
type Runner interface {
	Run(ctx context.Context, s domain.Step, progress func(domain.StepProgress)) domain.StepResult
}

// Terminal hands the real terminal to a child process, for the one interactive step.
type Terminal interface {
	Run(ctx context.Context, cmd *exec.Cmd) error
}
