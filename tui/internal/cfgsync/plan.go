package cfgsync

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// repoTimesFileKey is the key gitx.RepoTimes uses for a file entry: the entry
// path relative to itself.
const repoTimesFileKey = "."

func (s *Service) PlanSync(ctx context.Context, dir plan.Direction, entries []manifest.ResolvedEntry) (SyncPlan, error) {
	state, warning := s.LoadState(ctx)
	managed := make([]manifest.Entry, len(entries))
	for i, e := range entries {
		managed[i] = e.Entry
	}
	syncPlan := SyncPlan{Direction: dir, Selection: Selection{}, StateWarning: warning}
	for _, e := range entries {
		src, dst := e.RepoPath, e.DestPath
		if dir == plan.Pull {
			src, dst = dst, src
		}
		items, err := s.PlanEntry(ctx, dir, e, src, dst, managed)
		if errors.Is(err, errSourceMissing) {
			syncPlan.Notices = append(syncPlan.Notices, fmt.Sprintf("skipped: source missing: %s/%s", e.Root, e.Path))
			continue
		}
		if err != nil {
			return SyncPlan{}, err
		}
		for _, it := range items {
			syncPlan.Items = append(syncPlan.Items, it)
			syncPlan.Selection[it.Key()] = !it.Blocked && !slices.Contains(state.Ignores, ignoreKey(it.Key()))
		}
	}
	return syncPlan, nil
}

func ignoreKey(k ItemKey) string { return string(k.Root) + ":" + k.Path }

var errSourceMissing = errors.New("source missing")

// PlanEntry diffs one entry and turns the changes into plan items.
func (s *Service) PlanEntry(ctx context.Context, dir plan.Direction, e manifest.ResolvedEntry, src, dst string, managed []manifest.Entry) ([]PlanItem, error) {
	srcKind, err := s.Inspector.PathKind(src)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", e.Root, e.Path, err)
	}
	if srcKind == plan.KindNone {
		return nil, fmt.Errorf("%s/%s: source %s: %w", e.Root, e.Path, src, errSourceMissing)
	}
	changes, err := s.Inspector.Diff(ctx, e, plan.Endpoint{Path: src}, plan.Endpoint{Path: dst})
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", e.Root, e.Path, err)
	}
	fileEntry := srcKind == plan.KindFile || srcKind == plan.KindLink
	kept := changes[:0:0]
	for _, c := range changes {
		if !manifest.Excluded(c.Path, c.Kind == plan.KindDir, e.Filter) {
			c.Entry = e.Entry
			kept = append(kept, c)
		}
	}
	blocked := PreflightBlocked(dst, srcKind, kept, s.Inspector.PathKind)

	var repoTimes plan.RepoTimes
	if dir != plan.Restore && len(kept) > 0 {
		repoTimes, err = s.Inspector.RepoTimes(ctx, s.RepoDir, string(e.Root)+"/"+e.Path)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", e.Root, e.Path, err)
		}
	}

	items := make([]PlanItem, 0, len(kept))
	for _, c := range kept {
		it := PlanItem{Change: c, Resolved: e, Src: src, Dst: dst, Blocked: slices.Contains(blocked, c.Path), FileEntry: fileEntry}
		if dir != plan.Restore {
			s.attachTimes(ctx, &it, dir, repoTimes)
			it.Change.Warning = s.linkWarning(it, managed)
		}
		items = append(items, it)
	}
	return items, nil
}

// attachTimes sets the times of the sides that have the file: the source side
// except for a Delete, the destination side except for a Create.
func (s *Service) attachTimes(ctx context.Context, it *PlanItem, dir plan.Direction, repoTimes plan.RepoTimes) {
	onSrc := it.Change.Type != plan.Delete
	onDst := it.Change.Type != plan.Create
	repoHas, pcHas := onSrc, onDst
	if dir == plan.Pull {
		repoHas, pcHas = onDst, onSrc
	}
	if repoHas {
		key := it.Change.Path
		if it.FileEntry {
			key = repoTimesFileKey
		}
		it.RepoTime = repoTimes.Files[key]
	}
	if pcHas {
		pcPath := it.DstPath()
		if dir == plan.Pull {
			pcPath = it.srcPath()
		}
		it.PCTime, _ = s.Inspector.NewestMTime(ctx, pcPath, manifest.Filter{})
	}
}

// linkWarning classifies the source-side target of a created or updated link.
func (s *Service) linkWarning(it PlanItem, managed []manifest.Entry) plan.LinkWarning {
	if it.Change.Kind != plan.KindLink || it.Change.Type == plan.Delete {
		return plan.NoLinkWarning
	}
	target, err := s.Inspector.LinkTarget(it.srcPath())
	if err != nil {
		return plan.WarnLeavesManagedSet
	}
	return ClassifyLink(it.Change.Entry.Root, path.Dir(it.rel()), target, managed)
}
