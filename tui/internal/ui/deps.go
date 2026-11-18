package ui

import (
	"context"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/drift"
	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
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

	Drift        func(ctx context.Context, entries []manifest.ResolvedEntry) <-chan drift.DriftResult
	PlanSync     func(ctx context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (cfgsync.SyncPlan, error)
	PlanHard     func(ctx context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (cfgsync.HardPlan, error)
	NeedsBackup  func(p cfgsync.SyncPlan, sel cfgsync.Selection) bool
	ApplySync    func(ctx context.Context, p cfgsync.SyncPlan, sel cfgsync.Selection, opts cfgsync.ApplyOptions, progress func(plan.StepProgress)) cfgsync.ApplyResult
	ListBackups  func(ctx context.Context) ([]backup.Backup, error)
	PlanRestore  func(ctx context.Context, b backup.Backup) (cfgsync.SyncPlan, error)
	DeleteBackup func(ctx context.Context, dir string, confirmed bool) (plan.StepResult, bool)
	// ReadSide reads one end of a plan item for the resolver's row state and diff view.
	ReadSide func(path string) (cfgsync.Side, error)
	Edit     func(ctx context.Context, req cfgsync.EditRequest) (cfgsync.EditResult, error)

	Install  InstallDeps
	Packages PackagesDeps
}

// InstallDeps is what the Install section needs besides the shell.
type InstallDeps struct {
	Plan       func(ctx context.Context, d install.Draft) (install.InstallPlan, error)
	Run        func(ctx context.Context, p install.InstallPlan, events chan<- plan.Event, ack <-chan bool) install.InstallResult
	DetectGPUs func(ctx context.Context) ([]install.PCIDevice, error)
	// PathKind checks whether a path exists without opening it.
	PathKind func(path string) (plan.NodeKind, error)
	// Preview renders a planned step as a dry run; the summary shows its detail.
	Preview     func(ctx context.Context, step plan.Step) plan.StepResult
	SecretsPath string
	SecretNames []string
	Catalog     []manifest.Package
	Drivers     []manifest.Driver
}

// PackagesDeps is what the Packages section needs besides the shell.
type PackagesDeps struct {
	ListUnregistered func(ctx context.Context) ([]string, error)
	AddPackage       func(ctx context.Context, pkg manifest.Package) (plan.StepResult, error)
	IgnorePackage    func(ctx context.Context, name string) plan.StepResult
}
