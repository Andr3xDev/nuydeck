package app

import (
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// ProfileCustom selects exactly the packages named in Draft.Packages.
const ProfileCustom = "custom"

// Installer plans and runs the guided install.
// Plan uses Service, Probe, Now and the catalog fields; Run uses only Runner, Now, KeepaliveEvery and DryRun.
type Installer struct {
	Service *Service
	Probe   SystemProbe
	Runner  plan.Runner
	// Preview is always a Recorder; it renders the summary over the plan that Run executes.
	Preview        plan.Runner
	Now            func() time.Time
	KeepaliveEvery time.Duration
	DryRun         bool

	Catalog []manifest.Package
	Drivers []manifest.Driver
	Entries []manifest.ResolvedEntry
	Home    string
	User    string
}

// Draft is the wizard state the plan is built from.
type Draft struct {
	// Profile is minimal, full or custom.
	Profile        string
	Packages       []string
	Drivers        []string
	SkipGPUEnvLink bool
	SkipSecrets    bool
	Secrets        []plan.SecretEntry
	// ThemeVariant defaults to nadir when empty.
	ThemeVariant string
}

// InstallPlan is the ordered step list plus what the summary must show besides it.
type InstallPlan struct {
	Steps   []domain.PlannedStep
	Notices []string
	// Blocking errors forbid running the plan.
	Blocking []error
}

type InstallResult struct {
	Err error
}
