package main

import (
	"context"
	"path/filepath"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/drift"
	"github.com/Andr3xDev/nuydeck/tui/internal/dryrun"
	"github.com/Andr3xDev/nuydeck/tui/internal/gitx"
	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/packages"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/rsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
	"github.com/Andr3xDev/nuydeck/tui/internal/sysx"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui"
)

// Inspector is the real cfgsync.Inspector.
type Inspector struct {
	Home string
}

func (i Inspector) Diff(ctx context.Context, e manifest.ResolvedEntry, src, dst plan.Endpoint) ([]plan.Change, error) {
	kind, err := osfs.PathKind(src.Path)
	if err != nil {
		return nil, err
	}
	res, err := rsync.Plan(ctx, rsync.Request{Src: src.Path, Dst: dst.Path, Dir: kind == plan.KindDir, Filter: e.Filter})
	if err != nil {
		return nil, err
	}
	changes := make([]plan.Change, len(res.Items))
	for n, it := range res.Items {
		changes[n] = plan.Change{Entry: e.Entry, Path: it.Path, Type: it.Type, Kind: it.Kind}
	}
	return changes, nil
}

func (Inspector) LinkTarget(path string) (string, error)      { return osfs.LinkTarget(path) }
func (Inspector) PathKind(path string) (plan.NodeKind, error) { return osfs.PathKind(path) }

func (Inspector) NewestMTime(_ context.Context, path string, filter manifest.Filter) (time.Time, error) {
	return osfs.NewestMTime(path, filter)
}

func (Inspector) RepoTimes(ctx context.Context, repo, rel string) (plan.RepoTimes, error) {
	return gitx.Reader{}.RepoTimes(ctx, repo, rel)
}

func (i Inspector) ListBackups(ctx context.Context) ([]backup.Backup, error) {
	return backup.ListBackups(ctx, i.Home)
}

func (i Inspector) LoadState(context.Context) (state.State, string) {
	return state.LoadState(state.StatePath(i.Home))
}

// wireSync binds the drift, sync and backup use cases to the funcs the dashboard, sync and backups screens call.
func wireSync(deps *ui.Deps, svc *cfgsync.Service, backups *backup.Service, drifter *drift.Service) {
	deps.Drift = drifter.Drift
	deps.PlanSync = svc.PlanSync
	deps.NeedsBackup = svc.NeedsBackup
	deps.ApplySync = svc.Apply
	deps.ListBackups = backups.ListBackups
	deps.PlanRestore = backups.PlanRestore
	deps.DeleteBackup = backups.DeleteBackup
}

// wireCatalog loads the package, hardware and secret-name manifests and builds the install and packages use cases.
// A bad manifest only sets CatalogError, so sync and backups keep working.
func wireCatalog(deps *ui.Deps, svc *cfgsync.Service, repo, home, username string, dryRun bool, runner plan.Runner, inspector Inspector) {
	catalog, err := manifest.LoadPackages(filepath.Join(repo, packagesRelPath))
	if err != nil {
		deps.CatalogError = err
		return
	}
	drivers, err := manifest.LoadHardware(filepath.Join(repo, hardwareRelPath))
	if err != nil {
		deps.CatalogError = err
		return
	}
	secretNames, err := manifest.LoadSecrets(filepath.Join(repo, secretsRelPath))
	if err != nil {
		deps.CatalogError = err
		return
	}

	probe := sysx.Probe{User: username}
	preview := dryrun.Recorder{}
	installer := &install.Installer{
		Service: svc, Probe: probe, Runner: runner, Preview: preview, Now: time.Now, DryRun: dryRun,
		Catalog: catalog, Drivers: drivers, Entries: deps.Entries, Home: home, User: username,
	}
	deps.Install = ui.InstallDeps{
		Plan:       installer.Plan,
		Run:        installer.Run,
		DetectGPUs: probe.DetectGPUs,
		PathKind:   inspector.PathKind,
		Preview: func(ctx context.Context, step plan.Step) plan.StepResult {
			return preview.Run(ctx, step, nil)
		},
		SecretsPath: filepath.Join(home, ".secrets"),
		SecretNames: secretNames,
		Catalog:     catalog,
		Drivers:     drivers,
	}
	pkgs := &packages.Packages{
		Probe: probe, LoadState: inspector.LoadState, Runner: runner, Catalog: catalog, Drivers: drivers,
		CatalogPath: filepath.Join(repo, packagesRelPath),
	}
	deps.Packages = ui.PackagesDeps{
		ListUnregistered: pkgs.ListUnregistered,
		AddPackage:       pkgs.AddPackage,
		IgnorePackage:    pkgs.IgnorePackage,
	}
}
