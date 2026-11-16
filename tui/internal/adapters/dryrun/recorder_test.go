package dryrun

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

var _ app.Runner = Recorder{}

func TestRecorderRendersEveryStepWithoutIO(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cases := []struct {
		step domain.Step
		want string
	}{
		{domain.CopyStep{Src: "/a", Dst: "/b", Paths: []string{"x"}}, "would copy /a -> /b: x"},
		{domain.DeleteStep{Path: "/b/x"}, "would delete /b/x"},
		{domain.BackupStep{Files: []domain.BackupFile{{Root: "config", Path: "hypr/a"}}}, "would back up config/hypr/a"},
		{domain.DeleteBackupStep{Dir: "/bk/1"}, "would delete backup /bk/1"},
		{domain.SaveStateStep{}, "would save state"},
		{domain.AppendLogStep{Entry: state.LogEntry{Op: "push"}}, "would append log entry"},
	}
	for _, c := range cases {
		res := Recorder{}.Run(context.Background(), c.step, nil)
		if res.Status != domain.StepWouldApply || !strings.HasPrefix(res.Detail, c.want) {
			t.Errorf("%T: got %+v, want prefix %q", c.step, res, c.want)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("recorder created files: %v", entries)
	}
}
