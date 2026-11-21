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

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/drift"
	"github.com/Andr3xDev/nuydeck/tui/internal/dryrun"
	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/osexec"
	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui"
)

const (
	manifestRelPath = "manifest/paths.toml"
	packagesRelPath = "manifest/packages.toml"
	hardwareRelPath = "manifest/hardware.toml"
	secretsRelPath  = "manifest/secrets.toml"

	servicesRelPath   = "manifest/services.toml"
	referencesRelPath = "manifest/references.toml"
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
	svc := &cfgsync.Service{Inspector: inspector, LoadState: inspector.LoadState, Runner: runner, RepoDir: repo, Now: time.Now}

	backups := &backup.Service{Service: svc, Lister: inspector}
	drifter := &drift.Service{Service: svc, Inspector: inspector}

	deps := ui.Deps{Terminal: bridge, StateWarning: stateWarning, LastSync: state.LastSync}
	wireSync(&deps, svc, backups, drifter)
	wireEdit(&deps, bridge)
	wireRunPlan(&deps, runner)
	wireConfig(&deps, repo, home)
	if repo == "" {
		deps.ManifestError = errRepoNotFound
	} else if deps.Entries, err = loadEntries(repo, home); err != nil {
		deps.ManifestError = err
	} else {
		svc.AllEntries = deps.Entries
		wireCatalog(&deps, svc, repo, home, current.Username, dryRun, runner, inspector)
		wireServices(&deps, repo, current.Username)
		wireEnv(&deps, runner)
		wireReferences(&deps, repo, home)
	}

	root := ui.NewRoot(osfs.ReadVariant(configDir), dryRun, deps)
	program := tea.NewProgram(root)
	bridge.Attach(program)
	_, err = program.Run()
	bridge.Detach()
	root.WaitInstall(installShutdownTimeout)
	return err
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
	return manifest.Resolve(loaded, repo), nil
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
