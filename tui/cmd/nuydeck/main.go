package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/dryrun"
	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/osexec"
	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/sysx"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui"
)

const (
	manifestRelPath = "manifest/paths.toml"
	packagesRelPath = "manifest/packages.toml"
	hardwareRelPath = "manifest/hardware.toml"
	secretsRelPath  = "manifest/secrets.toml"
)

// installShutdownTimeout bounds the wait for a cancelled install to persist its log and reap its child.
const installShutdownTimeout = 10 * time.Second

var errRepoNotFound = errors.New("repo not found: pass --repo or run from inside the repo")

func main() {
	dryRun := flag.Bool("dry-run", false, "preview only: no file, state or log is written")
	repoFlag := flag.String("repo", "", "path to the nuydeck repo (default: saved state, then the working directory)")
	flag.Parse()

	if err := run(*dryRun, *repoFlag); err != nil {
		fmt.Fprintln(os.Stderr, "nuydeck:", err)
		os.Exit(1)
	}
}

func run(dryRun bool, repoFlag string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	logRsyncVersion()

	current, err := user.Current()
	if err != nil {
		return err
	}

	inspector := Inspector{Home: home}
	state, stateWarning := inspector.LoadState(context.Background())
	repo := resolveRepo(repoFlag, state.RepoPath, cwd)

	bridge := ui.NewTerminalBridge()
	var runner plan.Runner = osexec.Runner{Home: home, ConfigDir: configDir, Terminal: bridge}
	if dryRun {
		runner = dryrun.Recorder{}
	}
	svc := &app.Service{
		Service:   &cfgsync.Service{Inspector: inspector, LoadState: inspector.LoadState, Runner: runner, RepoDir: repo, Now: time.Now},
		Inspector: inspector,
	}

	deps := ui.Deps{Drifter: svc, Syncer: svc, Backups: svc, StateWarning: stateWarning, LastSync: state.LastSync}
	if repo == "" {
		deps.ManifestError = errRepoNotFound
	} else if deps.Entries, err = loadEntries(repo, home); err != nil {
		deps.ManifestError = err
	} else {
		wireCatalog(&deps, svc, repo, home, current.Username, dryRun, runner, inspector)
	}

	root := ui.NewRoot(osfs.ReadVariant(configDir), dryRun, deps)
	program := tea.NewProgram(root)
	bridge.Attach(program)
	_, err = program.Run()
	root.WaitInstall(installShutdownTimeout)
	return err
}

// wireCatalog loads the package, hardware and secret-name manifests and builds the install and packages use cases.
// A bad manifest only sets CatalogError, so sync and backups keep working.
func wireCatalog(deps *ui.Deps, svc *app.Service, repo, home, username string, dryRun bool, runner plan.Runner, inspector app.Inspector) {
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
	deps.Install = ui.InstallDeps{
		Installer: &app.Installer{
			Service: svc, Probe: probe, Runner: runner, Preview: preview, Now: time.Now, DryRun: dryRun,
			Catalog: catalog, Drivers: drivers, Entries: deps.Entries, Home: home, User: username,
		},
		Preview:     preview,
		GPUs:        probe,
		Paths:       inspector,
		SecretsPath: filepath.Join(home, ".secrets"),
		SecretNames: secretNames,
		Catalog:     catalog,
		Drivers:     drivers,
	}
	deps.Packages = &app.Packages{
		Probe: probe, Inspector: inspector, Runner: runner, Catalog: catalog, Drivers: drivers,
		CatalogPath: filepath.Join(repo, packagesRelPath),
	}
}

// resolveRepo picks the repo: the --repo flag, then the saved state, then the
// nearest ancestor of cwd holding the manifest. It returns "" when none is found.
// A saved repo without a manifest is stale and falls through to the cwd walk.
func resolveRepo(flagRepo, stateRepo, cwd string) string {
	if flagRepo != "" {
		if abs, err := filepath.Abs(flagRepo); err == nil {
			return abs
		}
		return flagRepo
	}
	if stateRepo != "" {
		if _, err := os.Stat(filepath.Join(stateRepo, manifestRelPath)); err == nil {
			return stateRepo
		}
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, manifestRelPath)); err == nil {
			return dir
		}
		if dir == filepath.Dir(dir) {
			return ""
		}
	}
}

func loadEntries(repo, home string) ([]manifest.ResolvedEntry, error) {
	loaded, err := manifest.LoadManifest(filepath.Join(repo, manifestRelPath), home)
	if err != nil {
		return nil, err
	}
	var entries []manifest.ResolvedEntry
	for _, e := range loaded.Entries {
		filter, whole := manifest.FiltersFor(e, loaded.Excludes)
		if whole {
			continue
		}
		entries = append(entries, manifest.ResolvedEntry{
			Entry:    e,
			RepoPath: filepath.Join(repo, string(e.Root), e.Path),
			DestPath: filepath.Join(loaded.Roots[e.Root], e.Path),
			Filter:   filter,
		})
	}
	return entries, nil
}

// logRsyncVersion prints the rsync version line to stderr before the TUI takes the screen.
func logRsyncVersion() {
	var out strings.Builder
	if _, err := execx.Run(context.Background(), &out, "rsync", "--version"); err != nil {
		fmt.Fprintln(os.Stderr, "nuydeck: rsync not available:", err)
		return
	}
	first, _, _ := strings.Cut(out.String(), "\n")
	fmt.Fprintln(os.Stderr, "nuydeck:", strings.TrimSpace(first))
}
