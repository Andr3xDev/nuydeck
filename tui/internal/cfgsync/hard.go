package cfgsync

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// HardPlan is a sync plan that also deletes: every offered item is selected, deletions included.
type HardPlan struct {
	SyncPlan
	Selection Selection
	// Confirm is the text the user must accept before Apply runs.
	Confirm string
}

// PlanHard plans a hard reinstall of the entries. Excluded paths never reach the plan, ignored and
// blocked items stay unselected, and entries with a missing source are skipped, never wiped.
func PlanHard(ctx context.Context, s *Service, dir plan.Direction, entries []manifest.ResolvedEntry) (HardPlan, error) {
	p, err := s.PlanSync(ctx, dir, entries)
	if err != nil {
		return HardPlan{}, err
	}
	for _, it := range p.Items {
		p.Selection[it.Key()] = !it.Blocked && !it.Ignored
	}
	return HardPlan{SyncPlan: p, Selection: p.Selection, Confirm: HardConfirm(p)}, nil
}

// HardConfirm lists what a hard run overwrites, deletes and leaves alone.
func HardConfirm(p SyncPlan) string {
	from, to, deleted := "repo", "PC", "the PC"
	if p.Direction == plan.Pull {
		from, to, deleted = "PC", "repo", "the repo"
	}
	var deletes, untouched []string
	overwrites := 0
	for _, it := range p.Items {
		key := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return '?'
			}
			return r
		}, ignoreKey(it.Key()))
		switch {
		case it.Ignored || it.Blocked:
			untouched = append(untouched, key)
		case !p.Selection[it.Key()]:
		case it.Change.Type != plan.Delete:
			overwrites++
		case it.Change.Kind == plan.KindDir:
			deletes = append(deletes, "delete "+key+" (directory)")
		default:
			deletes = append(deletes, "delete "+key)
		}
	}

	lines := []string{fmt.Sprintf("Hard reinstall: %s -> %s", from, to), fmt.Sprintf("Overwrite or create: %d", overwrites)}
	if len(deletes) > 0 {
		lines = append(lines, deletes...)
		lines = append(lines, "Deleting from "+deleted+": the paths listed above.")
	}
	lines = append(lines, p.Notices...)
	if len(untouched) > 0 {
		lines = append(lines, "Not touched (ignored or blocked):")
		lines = append(lines, untouched...)
	}
	return strings.Join(lines, "\n")
}
