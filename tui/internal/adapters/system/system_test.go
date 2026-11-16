package system_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/system"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunnerCopyStepUsesRsync(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not installed")
	}
	r := system.Runner{Home: t.TempDir(), ConfigDir: t.TempDir()}
	root := t.TempDir()
	ctx := context.Background()

	writeFile(t, root+"/repo/file", "new")
	writeFile(t, root+"/home/file", "old")
	res := r.Run(ctx, plan.CopyStep{Src: root + "/repo/file", Dst: root + "/home/file"}, nil)
	if res.Status != plan.StepOK || readFile(t, root+"/home/file") != "new" {
		t.Errorf("file copy: %+v", res)
	}

	writeFile(t, root+"/repo/dir/a", "a")
	var progressed []string
	res = r.Run(ctx, plan.CopyStep{Src: root + "/repo/dir", Dst: root + "/home/dir", Dir: true, Paths: []string{"a"}},
		func(p plan.StepProgress) { progressed = append(progressed, p.Path) })
	if res.Status != plan.StepOK || readFile(t, root+"/home/dir/a") != "a" {
		t.Errorf("dir copy: %+v", res)
	}
	if len(progressed) != 1 || progressed[0] != "a" {
		t.Errorf("progress = %v, want [a]", progressed)
	}
}

func TestRunnerUnsupportedStepFails(t *testing.T) {
	res := system.Runner{Home: t.TempDir(), ConfigDir: t.TempDir()}.Run(context.Background(), nil, nil)
	if res.Status != plan.StepFailed || res.Err == nil {
		t.Errorf("unsupported step: %+v", res)
	}
}

func TestRunnerBackupAndDeleteBackupDelegateToFsx(t *testing.T) {
	home, src := t.TempDir(), t.TempDir()
	writeFile(t, src+"/f", "data")
	r := system.Runner{Home: home, ConfigDir: t.TempDir()}
	ctx := context.Background()

	res := r.Run(ctx, plan.BackupStep{
		Direction: plan.Pull,
		Files:     []plan.BackupFile{{Root: "home", Path: "f"}},
		Roots:     map[manifest.Root]string{"home": src},
	}, nil)
	if res.Status != plan.StepOK || res.Detail == "" {
		t.Fatalf("backup: %+v", res)
	}
	if _, err := os.Stat(res.Detail); err != nil {
		t.Fatalf("backup dir missing: %v", err)
	}

	if del := r.Run(ctx, plan.DeleteBackupStep{Dir: res.Detail}, nil); del.Status != plan.StepOK {
		t.Fatalf("delete backup: %+v", del)
	}
	if _, err := os.Stat(res.Detail); !os.IsNotExist(err) {
		t.Errorf("backup dir still exists: %v", err)
	}
}

func TestRunnerDispatch(t *testing.T) {
	home, cfg := t.TempDir(), t.TempDir()
	r := system.Runner{Home: home, ConfigDir: cfg}
	ctx := context.Background()

	res := r.Run(ctx, plan.SaveStateStep{State: state.State{RepoPath: "/elsewhere/repo"}}, nil)
	if res.Status != plan.StepOK {
		t.Fatalf("save state: %+v", res)
	}
	st, warn := system.Inspector{Home: home}.LoadState(ctx)
	if warn != "" || st.RepoPath != "/elsewhere/repo" {
		t.Errorf("state = %+v, warning %q", st, warn)
	}

	entry := state.LogEntry{Time: time.Now().Format(time.RFC3339), Op: "push", Result: "ok"}
	if res := r.Run(ctx, plan.AppendLogStep{Entry: entry}, nil); res.Status != plan.StepOK {
		t.Fatalf("append log: %+v", res)
	}
	logs, _ := filepath.Glob(filepath.Join(cfg, "nuydeck", "logs", "*.jsonl"))
	if len(logs) != 1 {
		t.Errorf("log files = %v", logs)
	}

	victim := filepath.Join(home, "victim")
	if err := os.WriteFile(victim, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if res := r.Run(ctx, plan.DeleteStep{Path: victim, Root: home}, nil); res.Status != plan.StepOK {
		t.Fatalf("delete: %+v", res)
	}
	if _, err := os.Lstat(victim); !os.IsNotExist(err) {
		t.Errorf("victim still exists: %v", err)
	}
}

func TestAppendPackageStep_AppendsThroughTomlx(t *testing.T) {
	path := filepath.Join(t.TempDir(), "packages.toml")
	if err := os.WriteFile(path, []byte("version = 1\n\n[[package]]\nname = \"a\"\nsource = \"pacman\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := newRunner(&fakeExec{}).Run(context.Background(), plan.AppendPackageStep{Path: path, Entry: manifest.Package{Name: "htop", Source: manifest.SourceAUR}}, nil)
	if res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), `name = "htop"`) {
		t.Errorf("file = %q, want htop appended", b)
	}
}

func TestFileStepsDelegateToFsx(t *testing.T) {
	dir := t.TempDir()
	r := newRunner(&fakeExec{})
	ctx := context.Background()

	steps := map[string]plan.Step{
		"theme":   plan.WriteFileStep{Path: filepath.Join(dir, "theme", "name"), Content: []byte("nuytek"), Mode: 0o644},
		"link":    plan.SymlinkStep{Path: filepath.Join(dir, "gpu_env.lua"), Target: "gpu_env_amd.lua"},
		"secrets": plan.WriteSecretsStep{Path: filepath.Join(dir, ".secrets")},
	}
	for name, s := range steps {
		if res := r.Run(ctx, s, nil); res.Status != plan.StepOK {
			t.Errorf("%s: %+v", name, res)
		}
	}
}
