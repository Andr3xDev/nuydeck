package system_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/system"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func TestRunnerDispatch(t *testing.T) {
	home, cfg := t.TempDir(), t.TempDir()
	r := system.Runner{Home: home, ConfigDir: cfg}
	ctx := context.Background()

	res := r.Run(ctx, domain.SaveStateStep{State: domain.State{RepoPath: "/elsewhere/repo"}}, nil)
	if res.Status != domain.StepOK {
		t.Fatalf("save state: %+v", res)
	}
	st, warn := system.Inspector{Home: home}.LoadState(ctx)
	if warn != "" || st.RepoPath != "/elsewhere/repo" {
		t.Errorf("state = %+v, warning %q", st, warn)
	}

	entry := domain.LogEntry{Time: time.Now().Format(time.RFC3339), Op: "push", Result: "ok"}
	if res := r.Run(ctx, domain.AppendLogStep{Entry: entry}, nil); res.Status != domain.StepOK {
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
	if res := r.Run(ctx, domain.DeleteStep{Path: victim, Root: home}, nil); res.Status != domain.StepOK {
		t.Fatalf("delete: %+v", res)
	}
	if _, err := os.Lstat(victim); !os.IsNotExist(err) {
		t.Errorf("victim still exists: %v", err)
	}
}
