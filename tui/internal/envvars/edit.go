package envvars

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/editor"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// PlanEdit opens path in the user's editor, then logs the edit; the log holds the path, never file content.
func PlanEdit(getenv func(string) string, hasCommand func(string) bool, now func() time.Time, path string) (plan.Plan, error) {
	if _, err := os.Stat(path); err != nil {
		return plan.Plan{}, fmt.Errorf("env file: %w", err)
	}
	argv, err := editor.Resolve(getenv, hasCommand)
	if err != nil {
		return plan.Plan{}, err
	}
	label := fmt.Sprintf("edit env (%s)", filepath.Base(argv[0]))
	logStep := plan.AppendLogStep{}
	logStep.Entry.Time = now().Format(time.RFC3339)
	logStep.Entry.Op = label
	logStep.Entry.Result = "ok"
	logStep.Entry.Items = []state.LogItem{{Path: path, Kind: "file", Result: "edited"}}
	return plan.Plan{
		Title: label,
		Steps: []plan.Step{
			plan.CommandStep{Label: label, Argv: append(argv, path), Interactive: true},
			logStep,
		},
	}, nil
}
