package envvars_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/dryrun"
	"github.com/Andr3xDev/nuydeck/tui/internal/envvars"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

var fixedNow = func() time.Time { return time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC) }

func envFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "env_variables.lua")
	if err := os.WriteFile(path, []byte(`hl.env("SECRET_LOOKING", "hunter2")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func getenvOf(env map[string]string) func(string) string {
	return func(k string) string { return env[k] }
}

func installed(names ...string) func(string) bool {
	return func(n string) bool { return slices.Contains(names, n) }
}

func TestPlanEdit_BuildsInteractiveCommandThenLogStep(t *testing.T) {
	path := envFile(t)
	tests := []struct {
		name      string
		env       map[string]string
		installed []string
		wantArgv  []string
		wantLabel string
	}{
		{"visual with args", map[string]string{"VISUAL": "code -w"}, nil, []string{"code", "-w", path}, "edit env (code)"},
		{"editor absolute path uses base name", map[string]string{"EDITOR": "/usr/bin/nano"}, nil, []string{"/usr/bin/nano", path}, "edit env (nano)"},
		{"fallback", nil, []string{"vim"}, []string{"vim", path}, "edit env (vim)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := envvars.PlanEdit(getenvOf(tt.env), installed(tt.installed...), fixedNow, path)
			if err != nil {
				t.Fatalf("PlanEdit() error = %v", err)
			}
			if len(got.Steps) != 2 {
				t.Fatalf("PlanEdit() steps = %+v, want command then log", got.Steps)
			}
			cmd, ok := got.Steps[0].(plan.CommandStep)
			if !ok {
				t.Fatalf("step 0 = %T, want plan.CommandStep", got.Steps[0])
			}
			if cmd.Label != tt.wantLabel || !slices.Equal(cmd.Argv, tt.wantArgv) || !cmd.Interactive {
				t.Errorf("command = %+v, want label %q argv %v interactive", cmd, tt.wantLabel, tt.wantArgv)
			}
			logStep, ok := got.Steps[1].(plan.AppendLogStep)
			if !ok {
				t.Fatalf("step 1 = %T, want plan.AppendLogStep", got.Steps[1])
			}
			if logStep.Entry.Op != tt.wantLabel {
				t.Errorf("log Op = %q, want %q", logStep.Entry.Op, tt.wantLabel)
			}
		})
	}
}

func TestPlanEdit_LogRecordsPathAndNeverContent(t *testing.T) {
	path := envFile(t)
	got, err := envvars.PlanEdit(getenvOf(map[string]string{"EDITOR": "nvim"}), installed(), fixedNow, path)
	if err != nil {
		t.Fatal(err)
	}
	entry := got.Steps[1].(plan.AppendLogStep).Entry
	if len(entry.Items) != 1 || entry.Items[0].Path != path {
		t.Errorf("log items = %+v, want the single path %q", entry.Items, path)
	}
	if entry.Time != fixedNow().Format(time.RFC3339) {
		t.Errorf("log time = %q", entry.Time)
	}
	if strings.Contains(entry.Op, "hunter2") {
		t.Errorf("log Op leaks file content: %q", entry.Op)
	}
}

func TestPlanEdit_MissingCopyFailsBeforeResolvingEditor(t *testing.T) {
	getenvCalls := 0
	getenv := func(string) string { getenvCalls++; return "nvim" }
	_, err := envvars.PlanEdit(getenv, installed(), fixedNow, filepath.Join(t.TempDir(), "absent.lua"))
	if err == nil {
		t.Fatal("PlanEdit() error = nil, want error for missing copy")
	}
	if getenvCalls != 0 {
		t.Errorf("editor resolved %d times before the existence check", getenvCalls)
	}
}

func TestPlanEdit_NoEditorFails(t *testing.T) {
	_, err := envvars.PlanEdit(getenvOf(nil), installed(), fixedNow, envFile(t))
	if err == nil || !strings.Contains(err.Error(), "no editor") {
		t.Errorf("PlanEdit() error = %v, want no editor error", err)
	}
}

func TestPlanEdit_RecorderPreviewIsWouldRunEditorPath(t *testing.T) {
	path := envFile(t)
	got, err := envvars.PlanEdit(getenvOf(map[string]string{"EDITOR": "nvim"}), installed(), fixedNow, path)
	if err != nil {
		t.Fatal(err)
	}
	res := dryrun.Recorder{}.Run(context.Background(), got.Steps[0], nil)
	if want := "would run nvim " + path; res.Detail != want {
		t.Errorf("preview = %q, want %q", res.Detail, want)
	}
}
