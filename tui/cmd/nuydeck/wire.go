package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/drift"
	"github.com/Andr3xDev/nuydeck/tui/internal/dryrun"
	"github.com/Andr3xDev/nuydeck/tui/internal/envvars"
	"github.com/Andr3xDev/nuydeck/tui/internal/gitx"
	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/osexec"
	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/packages"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/references"
	"github.com/Andr3xDev/nuydeck/tui/internal/rsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/selfupdate"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
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

func (Inspector) ReadSide(path string) (cfgsync.Side, error) { return osfs.ReadSide(path) }

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
	deps.PlanHard = func(ctx context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (cfgsync.HardPlan, error) {
		return cfgsync.PlanHard(ctx, svc, dir, entries)
	}
	deps.NeedsBackup = svc.NeedsBackup
	deps.ApplySync = svc.Apply
	deps.ListBackups = backups.ListBackups
	deps.PlanRestore = backups.PlanRestore
	deps.PlanManualBackup = backups.PlanManual
	deps.ReadSide = osfs.ReadSide
}

// wireRunPlan binds the one executor every confirmed plan runs through; in dry-run the runner records instead of executing.
func wireRunPlan(deps *ui.Deps, runner plan.Runner) {
	deps.RunPlan = func(ctx context.Context, p plan.Plan, emit func(plan.Event)) error {
		return plan.Apply(ctx, runner, planItems(p.Steps), plan.Hooks{}, emit).Err
	}
}

// wireConfig fills the Config section, which stays usable without a manifest; without a repo there is nothing to update.
func wireConfig(deps *ui.Deps, repo, home string) {
	deps.Config = ui.ConfigDeps{RepoPath: repo, BackupsDir: backup.BackupsDir(home)}
	if repo != "" {
		deps.Config.PlanUpdate = func() plan.Plan { return selfupdate.Plan(repo) }
	}
}

// wireEdit binds the conflict editor to the real environment, the temp dir and the shared terminal.
func wireEdit(deps *ui.Deps, terminal plan.Terminal) {
	ports := cfgsync.EditPorts{
		Getenv:     os.Getenv,
		HasCommand: func(name string) bool { _, err := exec.LookPath(name); return err == nil },
		Terminal:   terminal,
		Temp:       osfs.TempFS{},
		ReadSide:   osfs.ReadSide,
	}
	deps.Edit = func(ctx context.Context, req cfgsync.EditRequest) (cfgsync.EditResult, error) {
		return cfgsync.ComposeEdit(ctx, ports, req)
	}
}

func planItems(steps []plan.Step) []plan.Item {
	items := make([]plan.Item, len(steps))
	for i, s := range steps {
		items[i] = plan.Item{Step: s}
	}
	return items
}

// wireServices loads the services catalog; a bad file only sets Services.Err, so the other sections keep working.
func wireServices(deps *ui.Deps, repo, username string) {
	catalog, err := services.Load(filepath.Join(repo, servicesRelPath))
	if err != nil {
		deps.Services.Err = err
		return
	}
	probe := sysx.Probe{User: username}
	deps.Services = ui.ServicesDeps{
		Catalog: catalog,
		State:   sysx.SystemctlProbe{}.Read,
		Plan: func(ctx context.Context, req services.Request, state services.UnitState) (plan.Plan, error) {
			return services.PlanAction(catalog, req, state, probe.CanSudoWithoutPrompt(ctx))
		},
	}
}

// wireEnv hands the env file to the editor through the runner, which in dry-run is the recorder: its line is the preview.
func wireEnv(deps *ui.Deps, runner plan.Runner) {
	hasCommand := sysx.Probe{}.HasCommand
	deps.Env.Edit = func(ctx context.Context, path string) (string, error) {
		p, err := envvars.PlanEdit(os.Getenv, hasCommand, time.Now, path)
		if err != nil {
			return "", err
		}
		var preview string
		hooks := plan.Hooks{After: func(_ context.Context, _ int, s plan.Step, r plan.StepResult) {
			if _, isCommand := s.(plan.CommandStep); isCommand && r.Status == plan.StepWouldApply {
				preview = r.Detail
			}
		}}
		return preview, plan.Apply(ctx, runner, planItems(p.Steps), hooks, func(plan.Event) {}).Err
	}
}

// wireReferences loads the references manifest; a bad file only sets References.Err.
func wireReferences(deps *ui.Deps, repo, home string) {
	catalog, err := references.Load(filepath.Join(repo, referencesRelPath))
	if err != nil {
		deps.References.Err = err
		return
	}
	hasCommand := sysx.Probe{}.HasCommand
	deps.References = ui.ReferencesDeps{
		Catalog: catalog,
		Status: func(ref references.Reference) references.Status {
			return references.Detect(ref, hasCommand, osfs.PathKind, home)
		},
		Open: osexec.OpenURL,
	}
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
		Dependencies: func(ctx context.Context) (map[string]bool, error) {
			names, err := probe.DependencyPackages(ctx)
			set := make(map[string]bool, len(names))
			for _, name := range names {
				set[name] = true
			}
			return set, err
		},
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
		List:          pkgs.List,
		AddPackage:    pkgs.AddPackage,
		IgnorePackage: pkgs.IgnorePackage,
	}
}
