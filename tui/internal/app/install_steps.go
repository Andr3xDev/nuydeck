package app

import (
	"context"
	"fmt"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

// pushSteps turns the push plan of the entries into a backup step (when something is overwritten) and one copy per entry.
// Delete items are never selected, so the install cannot remove owner files; a blocked item is a blocking plan error.
func (i *Installer) pushSteps(ctx context.Context, entries []manifest.ResolvedEntry) (steps []domain.Step, notices []string, blocking []error, err error) {
	plan, err := i.Service.PlanSync(ctx, domain.Push, entries)
	if err != nil {
		return nil, nil, nil, err
	}
	sel := Selection{}
	for _, it := range plan.Items {
		if it.Change.Type == domain.Delete {
			continue
		}
		if it.Blocked {
			blocking = append(blocking, fmt.Errorf("blocked by an existing path: %s", it.dstPath()))
		}
		sel[it.Key()] = plan.Selection[it.Key()]
	}
	items := selectedItems(plan, sel)
	if files := backupFiles(items); len(files) > 0 {
		steps = append(steps, backupStep(domain.Push, files))
	}
	for _, g := range groupByEntry(items) {
		paths, _ := copyPaths(g)
		steps = append(steps, domain.CopyStep{Entry: g[0].Resolved, Direction: domain.Push, Src: g[0].Src, Dst: g[0].Dst, Paths: paths, Dir: !g[0].fileEntry})
	}
	return steps, plan.Notices, blocking, nil
}
