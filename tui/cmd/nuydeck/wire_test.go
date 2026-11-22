package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/dryrun"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui"
)

func TestWireConfigOffersTheUpdateOnlyWithARepo(t *testing.T) {
	var deps ui.Deps

	wireConfig(&deps, "", "/home/u")
	if deps.Config.PlanUpdate != nil {
		t.Error("without a repo there is no update script to run")
	}

	wireConfig(&deps, "/repo", "/home/u")
	p := deps.Config.PlanUpdate()
	step, ok := p.Steps[0].(plan.CommandStep)
	if deps.Config.RepoPath != "/repo" || deps.Config.BackupsDir == "" || !ok || step.Argv[1] != filepath.Join("/repo", "update-tui.sh") {
		t.Errorf("config = %+v, step = %+v, want the repo path, a backups dir and the repo's update script", deps.Config, p.Steps)
	}
}

func TestWireRunPlanAppliesEveryStepThroughTheRunner(t *testing.T) {
	var deps ui.Deps
	wireRunPlan(&deps, dryrun.Recorder{})
	p := plan.Plan{Steps: []plan.Step{plan.CommandStep{Label: "a", Argv: []string{"true"}}}}

	var events []plan.Event
	if err := deps.RunPlan(context.Background(), p, func(e plan.Event) { events = append(events, e) }); err != nil || len(events) == 0 {
		t.Errorf("err = %v, events = %d, want a clean run that reports its step", err, len(events))
	}
}
