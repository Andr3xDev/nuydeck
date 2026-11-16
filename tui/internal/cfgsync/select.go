package cfgsync

import "github.com/Andr3xDev/nuydeck/tui/internal/plan"

// NeedsBackup reports whether the selection overwrites an existing destination file.
// Deletions are not counted: only a hard run deletes, and it forces its own backup.
func (s *Service) NeedsBackup(plan SyncPlan, sel Selection) bool {
	return len(BackupFiles(SelectedItems(plan, sel), false)) > 0
}

func SelectedItems(plan SyncPlan, sel Selection) []PlanItem {
	var items []PlanItem
	for _, it := range plan.Items {
		if sel[it.Key()] && !it.Blocked {
			items = append(items, it)
		}
	}
	return items
}

// BackupFiles lists the existing destination files the items overwrite, plus the ones they delete when hard.
func BackupFiles(items []PlanItem, hard bool) []PlanItem {
	var files []PlanItem
	for _, it := range items {
		if it.Change.Type != plan.Create && it.Change.Kind != plan.KindDir && (hard || it.Change.Type != plan.Delete) {
			files = append(files, it)
		}
	}
	return files
}
