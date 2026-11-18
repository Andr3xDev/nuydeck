package dryrun

import (
	"context"
	"os"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestRecorderRendersServiceAndConfirmSteps(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cases := []struct {
		step plan.Step
		want string
	}{
		{plan.ServiceActionStep{Unit: "NetworkManager.service", Scope: plan.ScopeSystem, Action: plan.ActionRestart}, "would run sudo systemctl restart NetworkManager.service"},
		{plan.ServiceActionStep{Unit: "waybar.service", Scope: plan.ScopeUser, Action: plan.ActionRestart}, "would run systemctl --user restart waybar.service"},
		{plan.ConfirmStep{ID: "spotify", Prompt: "Did Spotify open?"}, "would ask: Did Spotify open?"},
	}
	for _, c := range cases {
		res := Recorder{}.Run(context.Background(), c.step, nil)
		if res.Status != plan.StepWouldApply || res.Detail != c.want {
			t.Errorf("%T: got %+v, want detail %q", c.step, res, c.want)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("working dir changed: %v", entries)
	}
}
