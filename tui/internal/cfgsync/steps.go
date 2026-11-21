package cfgsync

import (
	"sort"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func directionName(d plan.Direction) string {
	switch d {
	case plan.Pull:
		return "pull"
	case plan.Restore:
		return "restore"
	case plan.Manual:
		return "manual"
	}
	return "push"
}

func kindName(k plan.NodeKind) string {
	switch k {
	case plan.KindDir:
		return "dir"
	case plan.KindLink:
		return "link"
	}
	return "file"
}

// BackupStep backs up files at the destination side; the roots are the destination root directories.
func BackupStep(dir plan.Direction, files []PlanItem) plan.BackupStep {
	step := plan.BackupStep{Direction: dir, Roots: map[manifest.Root]string{}}
	for _, it := range files {
		step.Files = append(step.Files, plan.BackupFile{Root: it.Change.Entry.Root, Path: it.rel()})
		step.Roots[it.Change.Entry.Root] = strings.TrimSuffix(it.Dst, "/"+it.Change.Entry.Path)
	}
	return step
}

func GroupByEntry(items []PlanItem) [][]PlanItem {
	var groups [][]PlanItem
	index := map[manifest.Entry]int{}
	for _, it := range items {
		i, ok := index[it.Change.Entry]
		if !ok {
			i = len(groups)
			index[it.Change.Entry] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], it)
	}
	return groups
}

// CopyPaths returns the paths to transfer and the items they cover.
func CopyPaths(group []PlanItem) ([]string, []PlanItem) {
	var paths []string
	var copied []PlanItem
	for _, it := range group {
		if it.Change.Type != plan.Delete {
			paths = append(paths, it.Change.Path)
			copied = append(copied, it)
		}
	}
	return paths, copied
}

// orderedDeletes lists file deletions in plan order, then directories deepest first.
func orderedDeletes(items []PlanItem) []PlanItem {
	var files, dirs []PlanItem
	for _, it := range items {
		switch {
		case it.Change.Type != plan.Delete:
		case it.Change.Kind == plan.KindDir:
			dirs = append(dirs, it)
		default:
			files = append(files, it)
		}
	}
	sort.SliceStable(dirs, func(a, b int) bool {
		return strings.Count(dirs[a].Change.Path, "/") > strings.Count(dirs[b].Change.Path, "/")
	})
	return append(files, dirs...)
}
