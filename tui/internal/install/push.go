package install

import (
	"context"
	"fmt"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// pushSteps turns the push plan of the entries into a backup step (when something is overwritten) and one copy per entry.
// Delete items are never selected, so the install cannot remove owner files; a blocked item is a blocking plan error.
func (i *Installer) pushSteps(ctx context.Context, entries []manifest.ResolvedEntry) (steps []plan.Step, notices []string, blocking []error, err error) {
	syncPlan, err := i.Service.PlanSync(ctx, plan.Push, entries)
	if err != nil {
		return nil, nil, nil, err
	}
	sel := cfgsync.Selection{}
	for _, it := range syncPlan.Items {
		if it.Change.Type == plan.Delete {
			continue
		}
		if it.Blocked {
			blocking = append(blocking, fmt.Errorf("blocked by an existing path: %s", it.DstPath()))
		}
		sel[it.Key()] = syncPlan.Selection[it.Key()]
	}
	for _, b := range cfgsync.Steps(syncPlan, sel, nil, cfgsync.ApplyOptions{Backup: true}) {
		steps = append(steps, b.Step)
	}
	return steps, syncPlan.Notices, blocking, nil
}
