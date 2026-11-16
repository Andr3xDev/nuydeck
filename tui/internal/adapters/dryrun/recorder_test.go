package dryrun

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

var _ plan.Runner = Recorder{}

func TestRecorderRendersEveryStepWithoutIO(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cases := []struct {
		step plan.Step
		want string
	}{
		{plan.CopyStep{Src: "/a", Dst: "/b", Paths: []string{"x"}}, "would copy /a -> /b: x"},
		{plan.DeleteStep{Path: "/b/x"}, "would delete /b/x"},
		{plan.BackupStep{Files: []plan.BackupFile{{Root: "config", Path: "hypr/a"}}}, "would back up config/hypr/a"},
		{plan.DeleteBackupStep{Dir: "/bk/1"}, "would delete backup /bk/1"},
		{plan.SaveStateStep{}, "would save state"},
		{plan.AppendLogStep{Entry: state.LogEntry{Op: "push"}}, "would append log entry"},
	}
	for _, c := range cases {
		res := Recorder{}.Run(context.Background(), c.step, nil)
		if res.Status != plan.StepWouldApply || !strings.HasPrefix(res.Detail, c.want) {
			t.Errorf("%T: got %+v, want prefix %q", c.step, res, c.want)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("recorder created files: %v", entries)
	}
}
