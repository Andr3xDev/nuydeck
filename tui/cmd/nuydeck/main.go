package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/dryrun"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/fsx"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/system"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/tomlx"
	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui"
)

const manifestRelPath = "manifest/paths.toml"

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

	inspector := system.Inspector{Home: home}
	state, stateWarning := inspector.LoadState(context.Background())
	repo := resolveRepo(repoFlag, state.RepoPath, cwd)

	var runner app.Runner = system.Runner{Home: home, ConfigDir: configDir}
	if dryRun {
		runner = dryrun.Recorder{}
	}
	svc := &app.Service{Inspector: inspector, Runner: runner, RepoDir: repo, Now: time.Now}

	deps := ui.Deps{Drifter: svc, Syncer: svc, Backups: svc, StateWarning: stateWarning, LastSync: state.LastSync}
	if repo == "" {
		deps.ManifestError = errRepoNotFound
	} else if deps.Entries, err = loadEntries(repo, home); err != nil {
		deps.ManifestError = err
	}

	_, err = tea.NewProgram(ui.NewRoot(fsx.ReadVariant(configDir), dryRun, deps)).Run()
	return err
}

// resolveRepo picks the repo: the --repo flag, then the saved state, then the
// nearest ancestor of cwd holding the manifest. It returns "" when none is found.
func resolveRepo(flagRepo, stateRepo, cwd string) string {
	if flagRepo != "" {
		return flagRepo
	}
	if stateRepo != "" {
		return stateRepo
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

func loadEntries(repo, home string) ([]domain.ResolvedEntry, error) {
	manifest, err := tomlx.LoadManifest(filepath.Join(repo, manifestRelPath), home)
	if err != nil {
		return nil, err
	}
	var entries []domain.ResolvedEntry
	for _, e := range manifest.Entries {
		filter, whole := domain.FiltersFor(e, manifest.Excludes)
		if whole {
			continue
		}
		entries = append(entries, domain.ResolvedEntry{
			Entry:    e,
			RepoPath: filepath.Join(repo, string(e.Root), e.Path),
			DestPath: filepath.Join(manifest.Roots[e.Root], e.Path),
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
