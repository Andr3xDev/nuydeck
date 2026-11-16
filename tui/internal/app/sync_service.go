package app

import (
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Service bundles the use cases over the ports.
type Service struct {
	Inspector Inspector
	Runner    plan.Runner
	RepoDir   string
	Now       func() time.Time
}
