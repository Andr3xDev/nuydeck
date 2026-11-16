package domain

import (
	"path"
	"strings"
)

// FiltersFor translates the excludes of the entry's root into rsync rules anchored at the entry;
// the bool reports that the whole entry is excluded.
func FiltersFor(entry Entry, excludes []Exclude) (Filter, bool) {
	var filter Filter
	for _, x := range excludes {
		if x.Root != entry.Root {
			continue
		}
		if matchesPrefix(entry.Path, strings.TrimSuffix(x.Pattern, "/")) {
			return Filter{}, true
		}
		if rest, ok := strings.CutPrefix(x.Pattern, entry.Path+"/"); ok {
			filter.Excludes = append(filter.Excludes, "/"+rest)
		}
	}
	return filter, false
}

// matchesPrefix reports whether p equals a path matching the glob pattern or lies below one; segments are matched one by one so * never crosses /.
func matchesPrefix(p, pattern string) bool {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(p, "/")
	if len(patternSegments) > len(pathSegments) {
		return false
	}
	for i, segment := range patternSegments {
		if ok, err := path.Match(segment, pathSegments[i]); err != nil || !ok {
			return false
		}
	}
	return true
}

// Excluded applies anchored rsync rules ("/a/*.log", trailing "/" = dirs only):
// a rule hits a path that matches it segment by segment or lies below a match.
func Excluded(rel string, isDir bool, f Filter) bool {
	relSegments := strings.Split(rel, "/")
	for _, rule := range f.Excludes {
		dirOnly := strings.HasSuffix(rule, "/")
		ruleSegments := strings.Split(strings.Trim(rule, "/"), "/")
		if len(ruleSegments) > len(relSegments) || (dirOnly && len(ruleSegments) == len(relSegments) && !isDir) {
			continue
		}
		matched := true
		for i, segment := range ruleSegments {
			if ok, err := path.Match(segment, relSegments[i]); err != nil || !ok {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
