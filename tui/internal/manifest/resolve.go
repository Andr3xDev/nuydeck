package manifest

import "path/filepath"

// Resolve anchors every entry to the repo and its destination root, dropping entries excluded as a whole.
func Resolve(m Manifest, repo string) []ResolvedEntry {
	var entries []ResolvedEntry
	for _, e := range m.Entries {
		filter, whole := FiltersFor(e, m.Excludes)
		if whole {
			continue
		}
		entries = append(entries, ResolvedEntry{
			Entry:    e,
			RepoPath: filepath.Join(repo, string(e.Root), e.Path),
			DestPath: filepath.Join(m.Roots[e.Root], e.Path),
			Filter:   filter,
		})
	}
	return entries
}
