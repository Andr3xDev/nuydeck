package envvars

import (
	"io/fs"
	"path/filepath"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// PlanToggle toggles v in content and plans the write of the chosen copy.
// The live copy is backed up first; the repo copy is not, git is its history.
func PlanToggle(source Source, loc Locations, content []byte, mode fs.FileMode, v Var) (plan.Plan, ToggleChange, error) {
	updated, change, err := ToggleLine(content, v)
	if err != nil {
		return plan.Plan{}, ToggleChange{}, err
	}
	path := loc.Path(source)
	var steps []plan.Step
	if source == SourceLive {
		steps = append(steps, plan.BackupStep{
			Direction: plan.Manual,
			Roots:     map[manifest.Root]string{manifest.RootConfig: loc.LiveRoot},
			Files:     []plan.BackupFile{{Root: manifest.RootConfig, Path: loc.LiveRel}},
		})
	}
	steps = append(steps, plan.WriteFileStep{Path: path, Content: updated, Mode: mode, Root: filepath.Dir(path)})
	return plan.Plan{Title: "toggle " + v.Name, Steps: steps}, change, nil
}
