// Package selfupdate plans the nuydeck self-update: pull the repo and rebuild the TUI binary.
package selfupdate

import (
	"path/filepath"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const scriptName = "update-tui.sh"

// Plan runs the repo's update script through bash, so the exec bit is not required.
func Plan(repo string) plan.Plan {
	return plan.Plan{
		Title:   "Update nuydeck",
		Confirm: "Pull the latest nuydeck and rebuild the TUI?",
		Notices: []string{
			"Repository: " + repo,
			"Restart nuydeck afterwards to run the new version.",
		},
		Steps: []plan.Step{plan.CommandStep{
			Label: "update nuydeck",
			Argv:  []string{"bash", filepath.Join(repo, scriptName)},
			Dir:   repo,
		}},
	}
}
