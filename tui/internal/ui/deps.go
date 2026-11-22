package ui

import (
	"context"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/drift"
	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/packages"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/references"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Deps is what the shell needs from the use cases and startup. A nil func hides the feature that needs it.
type Deps struct {
	Entries       []manifest.ResolvedEntry
	StateWarning  string
	LastSync      *state.LastSync
	ManifestError error
	// CatalogError replaces only the Install and Packages sections.
	CatalogError error

	// Terminal is the one interface field: the bridge is shared and attached to the program.
	Terminal plan.Terminal

	Drift       func(ctx context.Context, entries []manifest.ResolvedEntry) <-chan drift.DriftResult
	PlanSync    func(ctx context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (cfgsync.SyncPlan, error)
	PlanHard    func(ctx context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (cfgsync.HardPlan, error)
	NeedsBackup func(p cfgsync.SyncPlan, sel cfgsync.Selection) bool
	ApplySync   func(ctx context.Context, p cfgsync.SyncPlan, sel cfgsync.Selection, opts cfgsync.ApplyOptions, progress func(plan.StepProgress)) cfgsync.ApplyResult
	ListBackups func(ctx context.Context) ([]backup.Backup, error)
	PlanRestore func(ctx context.Context, b backup.Backup) (cfgsync.SyncPlan, error)
	// ReadSide reads one end of a plan item for the resolver's row state and diff view.
	ReadSide func(path string) (cfgsync.Side, error)
	Edit     func(ctx context.Context, req cfgsync.EditRequest) (cfgsync.EditResult, error)
	// RunPlan executes a confirmed plan, reporting each step to emit.
	RunPlan          func(ctx context.Context, p plan.Plan, emit func(plan.Event)) error
	PlanManualBackup func(ctx context.Context, entries []manifest.ResolvedEntry) (plan.Plan, error)

	Config     ConfigDeps
	Install    InstallDeps
	Packages   PackagesDeps
	Services   ServicesDeps
	Env        EnvDeps
	References ReferencesDeps
}

// ConfigDeps is what the Config section needs besides the shell.
type ConfigDeps struct {
	RepoPath   string
	BackupsDir string
	PlanUpdate func() plan.Plan
}

// ServicesDeps is what the Services section needs besides the shell. Err replaces only that section.
type ServicesDeps struct {
	Catalog []services.Entry
	Err     error
	// State reads the live state of the units of one scope.
	State func(ctx context.Context, scope plan.Scope, units []string) (map[string]services.UnitState, error)
	Plan  func(ctx context.Context, req services.Request, state services.UnitState) (plan.Plan, error)
}

// EnvDeps is what the Env section needs besides the shell and Deps.ReadSide.
type EnvDeps struct {
	// Edit hands the file to the editor; preview is the dry-run line, empty after a real edit.
	Edit func(ctx context.Context, path string) (preview string, err error)
}

// ReferencesDeps is what the References section needs. Err replaces only that section.
type ReferencesDeps struct {
	Catalog []references.Reference
	Err     error
	Status  func(ref references.Reference) references.Status
	Open    func(ctx context.Context, url string) error
}

// InstallDeps is what the Install section needs besides the shell.
type InstallDeps struct {
	Plan       func(ctx context.Context, d install.Draft) (install.InstallPlan, error)
	Run        func(ctx context.Context, p install.InstallPlan, events chan<- plan.Event, ack <-chan bool) install.InstallResult
	DetectGPUs func(ctx context.Context) ([]install.PCIDevice, error)
	// PathKind checks whether a path exists without opening it.
	PathKind func(path string) (plan.NodeKind, error)
	// Dependencies reports the installed packages marked as dependencies.
	Dependencies func(ctx context.Context) (map[string]bool, error)
	// Preview renders a planned step as a dry run; the summary shows its detail.
	Preview     func(ctx context.Context, step plan.Step) plan.StepResult
	SecretsPath string
	SecretNames []string
	Catalog     []manifest.Package
	Drivers     []manifest.Driver
}

// PackagesDeps is what the Packages section needs besides the shell.
type PackagesDeps struct {
	List          func(ctx context.Context) ([]packages.Row, error)
	AddPackage    func(ctx context.Context, pkg manifest.Package) (plan.StepResult, error)
	IgnorePackage func(ctx context.Context, name string) plan.StepResult
}
