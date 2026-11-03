package app

import (
	"context"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// Runner is the only way anything is modified; dry-run swaps in a recorder.
type Runner interface {
	Run(ctx context.Context, s domain.Step, progress func(domain.StepProgress)) domain.StepResult
}
