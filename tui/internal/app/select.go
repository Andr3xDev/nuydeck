package app

import "github.com/Andr3xDev/nuydeck/tui/internal/plan"

// NeedsBackup reports whether the selection overwrites or deletes an existing destination file.
func (s *Service) NeedsBackup(plan SyncPlan, sel Selection) bool {
	return len(backupFiles(selectedItems(plan, sel))) > 0
}

func selectedItems(plan SyncPlan, sel Selection) []PlanItem {
	var items []PlanItem
	for _, it := range plan.Items {
		if sel[it.Key()] && !it.Blocked {
			items = append(items, it)
		}
	}
	return items
}

func backupFiles(items []PlanItem) []PlanItem {
	var files []PlanItem
	for _, it := range items {
		if it.Change.Type != plan.Create && it.Change.Kind != plan.KindDir {
			files = append(files, it)
		}
	}
	return files
}
