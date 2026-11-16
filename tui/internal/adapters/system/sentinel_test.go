package system_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/system"
	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/dryrun"
	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const (
	secretSentinel = "SENTINEL-s3cr3t-Zx9Q-7731"
	activeMultilib = "[multilib]\nInclude = /etc/pacman.d/mirrorlist\n"
	pacmanConfPath = "/etc/pacman.conf"
)

type sentinelProbe struct{}

func (sentinelProbe) DetectGPUs(context.Context) ([]domain.PCIDevice, error) { return nil, nil }
func (sentinelProbe) InstalledPackages(context.Context) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func (sentinelProbe) ExplicitPackages(context.Context) ([]string, error) { return nil, nil }
func (sentinelProbe) HasCommand(string) bool                             { return true }
func (sentinelProbe) LoginShell(context.Context) (string, error)         { return "/usr/bin/zsh", nil }
func (sentinelProbe) IsRoot() bool                                       { return false }
func (sentinelProbe) ReadSystemFile(path string) ([]byte, error) {
	if path == pacmanConfPath {
		return []byte(activeMultilib), nil
	}
	return nil, fs.ErrNotExist
}

// capturingRunner keeps every StepResult the wrapped runner returns.
type capturingRunner struct {
	inner   plan.Runner
	mu      sync.Mutex
	results []plan.StepResult
}

func (c *capturingRunner) Run(ctx context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
	res := c.inner.Run(ctx, s, progress)
	c.mu.Lock()
	c.results = append(c.results, res)
	c.mu.Unlock()
	return res
}

// sentinelTerminal stands in for the sudo handoff so no command is launched.
type sentinelTerminal struct{}

func (sentinelTerminal) Run(context.Context, *exec.Cmd) error { return nil }

// sentinelExec answers every external command with success and one output line, so nothing real (sudo, pacman, git) runs.
func sentinelExec(_ context.Context, opts execx.Options, name string, _ ...string) (execx.Result, error) {
	if opts.Stdout != nil {
		fmt.Fprintf(opts.Stdout, "fake output of %s\n", name)
	}
	return execx.Result{}, nil
}

type sentinelRun struct {
	preview  []string
	events   []string
	results  []string
	plan     app.InstallPlan
	result   app.InstallResult
	outcomes []plan.StepResult
}

func sentinelInstaller(t *testing.T, home string, runner plan.Runner, dryRun bool) *app.Installer {
	t.Helper()
	inspector := system.Inspector{Home: home}
	now := func() time.Time { return time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC) }
	return &app.Installer{
		Service: &app.Service{Inspector: inspector, Runner: runner, RepoDir: filepath.Join(home, "repo"), Now: now},
		Probe:   sentinelProbe{},
		Runner:  runner,
		Preview: dryrun.Recorder{},
		Now:     now,
		DryRun:  dryRun,
		Catalog: []manifest.Package{{Name: "git", Source: manifest.SourcePacman, Profiles: []string{"minimal"}}},
		Home:    home,
		User:    "tester",
	}
}

// executeSentinelInstall plans, renders the preview and runs; beforeRun lets a test sabotage the target after Plan has read it.
func executeSentinelInstall(t *testing.T, inst *app.Installer, capture *capturingRunner, beforeRun func()) sentinelRun {
	t.Helper()
	plan, err := inst.Plan(context.Background(), app.Draft{
		Profile: "minimal",
		Secrets: []plan.SecretEntry{{Name: "API_TOKEN", Value: plan.Secret(secretSentinel)}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	out := sentinelRun{plan: plan}
	for _, ps := range plan.Steps {
		out.preview = append(out.preview, inst.Preview.Run(context.Background(), ps.Step, nil).Detail)
	}

	beforeRun()
	events := make(chan app.InstallEvent, 256)
	collected := make(chan struct{})
	go func() {
		defer close(collected)
		for e := range events {
			line := e.Line
			if e.Err != nil {
				line += " " + e.Err.Error()
			}
			out.events = append(out.events, line)
		}
	}()
	out.result = inst.Run(context.Background(), plan, events, nil)
	<-collected

	capture.mu.Lock()
	defer capture.mu.Unlock()
	out.outcomes = append(out.outcomes, capture.results...)
	for _, r := range capture.results {
		out.results = append(out.results, r.Detail)
		if r.Err != nil {
			out.results = append(out.results, r.Err.Error())
		}
	}
	return out
}

func (r sentinelRun) assertNoSentinel(t *testing.T) {
	t.Helper()
	surfaces := map[string][]string{
		"preview":  r.preview,
		"events":   r.events,
		"results":  r.results,
		"plan fmt": {fmt.Sprintf("%v|%+v|%#v", r.plan, r.plan, r.plan)},
		"result":   {fmt.Sprintf("%v|%+v|%#v", r.result, r.result, r.result)},
	}
	if r.result.Err != nil {
		surfaces["Err.Error"] = []string{r.result.Err.Error()}
	}
	for name, texts := range surfaces {
		for _, text := range texts {
			if strings.Contains(text, secretSentinel) {
				t.Errorf("sentinel leaked in %s: %q", name, text)
			}
		}
	}
}

func TestSentinelRecorderPlanAndRun(t *testing.T) {
	home := t.TempDir()
	capture := &capturingRunner{inner: dryrun.Recorder{}}
	inst := sentinelInstaller(t, home, capture, true)

	run := executeSentinelInstall(t, inst, capture, func() {})

	if run.result.Err != nil {
		t.Fatalf("dry-run failed: %v", run.result.Err)
	}
	if !strings.Contains(strings.Join(run.preview, "\n"), "API_TOKEN") {
		t.Fatalf("preview lacks the variable name, so the secrets step was not exercised: %q", run.preview)
	}
	run.assertNoSentinel(t)
}

// Test 2 runs the real system.Runner (real fsx secrets, file and log adapters); only external commands and the sudo handoff are faked.
func TestSentinelRealRunnerSecretsWriteFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only target is not enforced for root")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	configDir := filepath.Join(root, "config")
	stagingDir := filepath.Join(root, "staging")
	for _, dir := range []string{home, configDir, stagingDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runner := system.Runner{Home: home, ConfigDir: configDir, Exec: sentinelExec, Terminal: sentinelTerminal{}, TempDir: stagingDir}
	capture := &capturingRunner{inner: runner}
	inst := sentinelInstaller(t, home, capture, false)
	inst.KeepaliveEvery = time.Hour

	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })
	run := executeSentinelInstall(t, inst, capture, func() {
		if err := os.Chmod(home, 0o500); err != nil {
			t.Fatal(err)
		}
	})

	if run.result.Err == nil || !errors.Is(run.result.Err, fs.ErrPermission) {
		t.Fatalf("expected a permission failure on the secrets write, got %v", run.result.Err)
	}
	run.assertNoSentinel(t)

	logs, err := filepath.Glob(filepath.Join(state.LogsDir(configDir), "*"))
	if err != nil || len(logs) == 0 {
		t.Fatalf("Record step wrote no log file (glob %v): %v", logs, err)
	}
	// Covers the JSONL log and the state file wherever they live.
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if strings.Contains(path, secretSentinel) {
			t.Errorf("sentinel in path %s", path)
		}
		if d.IsDir() {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), secretSentinel) {
			t.Errorf("sentinel leaked into file %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
