package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// repoTimesFileKey is the key gitx.RepoTimes uses for a file entry: the entry
// path relative to itself.
const repoTimesFileKey = "."

// Service bundles the use cases over the ports.
type Service struct {
	Inspector Inspector
	Runner    Runner
	RepoDir   string
	Now       func() time.Time
}

// ItemKey identifies a plan item: the managed root and the root-relative path.
type ItemKey struct {
	Root domain.Root
	Path string
}

// Selection holds the keys of the items to apply.
type Selection map[ItemKey]bool

// PlanItem is one offered change with everything the preview shows.
// Src and Dst are the endpoints of the entry. For a file entry they are the
// file paths and Change.Path is the file's base name, as rsync itemizes it.
type PlanItem struct {
	Change   domain.Change
	Resolved domain.ResolvedEntry
	Src, Dst string
	Blocked  bool
	// RepoTime and PCTime are zero for the side that does not have the file.
	RepoTime, PCTime time.Time
	fileEntry        bool
}

// rel is the root-relative path of the item.
func (i PlanItem) rel() string {
	if i.fileEntry {
		return i.Change.Entry.Path
	}
	return path.Join(i.Change.Entry.Path, i.Change.Path)
}

func (i PlanItem) srcPath() string { return i.endpoint(i.Src) }
func (i PlanItem) dstPath() string { return i.endpoint(i.Dst) }

func (i PlanItem) endpoint(entryPath string) string {
	if i.fileEntry {
		return entryPath
	}
	return path.Join(entryPath, i.Change.Path)
}

// Key is the root-relative identity of the item (entry path joined with the change path).
func (i PlanItem) Key() ItemKey { return ItemKey{Root: i.Change.Entry.Root, Path: i.rel()} }

type SyncPlan struct {
	Direction    domain.Direction
	Items        []PlanItem
	Selection    Selection
	StateWarning string
}

type ApplyOptions struct {
	// Backup is true when the user accepted the backup offer.
	Backup bool
	DryRun bool
}

type StepOutcome struct {
	Step   domain.Step
	Result domain.StepResult
}

const (
	ItemApplied    = "applied"
	ItemNotApplied = "not applied"
)

type ItemResult struct {
	Key    ItemKey
	Result string
}

type ApplyResult struct {
	// Nothing is true when no step was needed; nothing ran and nothing was logged.
	Nothing  bool
	Outcomes []StepOutcome
	Items    []ItemResult
	Err      error
}

func (s *Service) PlanSync(ctx context.Context, dir domain.Direction, entries []domain.ResolvedEntry) (SyncPlan, error) {
	state, warning := s.Inspector.LoadState(ctx)
	managed := make([]domain.Entry, len(entries))
	for i, e := range entries {
		managed[i] = e.Entry
	}
	plan := SyncPlan{Direction: dir, Selection: Selection{}, StateWarning: warning}
	for _, e := range entries {
		src, dst := e.RepoPath, e.DestPath
		if dir == domain.Pull {
			src, dst = dst, src
		}
		items, err := s.planEntry(ctx, dir, e, src, dst, managed)
		if err != nil {
			return SyncPlan{}, err
		}
		for _, it := range items {
			plan.Items = append(plan.Items, it)
			plan.Selection[it.Key()] = !it.Blocked && !slices.Contains(state.Ignores, ignoreKey(it.Key()))
		}
	}
	return plan, nil
}

func ignoreKey(k ItemKey) string { return string(k.Root) + ":" + k.Path }

// planEntry diffs one entry and turns the changes into plan items.
func (s *Service) planEntry(ctx context.Context, dir domain.Direction, e domain.ResolvedEntry, src, dst string, managed []domain.Entry) ([]PlanItem, error) {
	changes, err := s.Inspector.Diff(ctx, e, domain.Endpoint{Path: src}, domain.Endpoint{Path: dst})
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", e.Root, e.Path, err)
	}
	srcKind, err := s.Inspector.PathKind(src)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", e.Root, e.Path, err)
	}
	if srcKind == domain.KindNone {
		return nil, fmt.Errorf("%s/%s: source %s does not exist", e.Root, e.Path, src)
	}
	fileEntry := srcKind == domain.KindFile || srcKind == domain.KindLink
	kept := changes[:0:0]
	for _, c := range changes {
		if !domain.Excluded(c.Path, c.Kind == domain.KindDir, e.Filter) {
			c.Entry = e.Entry
			kept = append(kept, c)
		}
	}
	blocked := domain.PreflightBlocked(dst, srcKind, kept, s.Inspector.PathKind)

	var repoTimes domain.RepoTimes
	if dir != domain.Restore && len(kept) > 0 {
		repoTimes, err = s.Inspector.RepoTimes(ctx, s.RepoDir, string(e.Root)+"/"+e.Path)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", e.Root, e.Path, err)
		}
	}

	items := make([]PlanItem, 0, len(kept))
	for _, c := range kept {
		it := PlanItem{Change: c, Resolved: e, Src: src, Dst: dst, Blocked: slices.Contains(blocked, c.Path), fileEntry: fileEntry}
		if dir != domain.Restore {
			s.attachTimes(ctx, &it, dir, repoTimes)
			it.Change.Warning = s.linkWarning(it, managed)
		}
		items = append(items, it)
	}
	return items, nil
}

// attachTimes sets the times of the sides that have the file: the source side
// except for a Delete, the destination side except for a Create.
func (s *Service) attachTimes(ctx context.Context, it *PlanItem, dir domain.Direction, repoTimes domain.RepoTimes) {
	onSrc := it.Change.Type != domain.Delete
	onDst := it.Change.Type != domain.Create
	repoHas, pcHas := onSrc, onDst
	if dir == domain.Pull {
		repoHas, pcHas = onDst, onSrc
	}
	if repoHas {
		key := it.Change.Path
		if it.fileEntry {
			key = repoTimesFileKey
		}
		it.RepoTime = repoTimes.Files[key]
	}
	if pcHas {
		pcPath := it.dstPath()
		if dir == domain.Pull {
			pcPath = it.srcPath()
		}
		it.PCTime, _ = s.Inspector.NewestMTime(ctx, pcPath, domain.Filter{})
	}
}

// linkWarning classifies the source-side target of a created or updated link.
func (s *Service) linkWarning(it PlanItem, managed []domain.Entry) domain.LinkWarning {
	if it.Change.Kind != domain.KindLink || it.Change.Type == domain.Delete {
		return domain.NoLinkWarning
	}
	target, err := s.Inspector.LinkTarget(it.srcPath())
	if err != nil {
		return domain.WarnLeavesManagedSet
	}
	return domain.ClassifyLink(it.Change.Entry.Root, path.Dir(it.rel()), target, managed)
}

// PlanRestore plans a restore from the backup; roots come from the backup metadata and deletions are never offered.
func (s *Service) PlanRestore(ctx context.Context, b domain.Backup) (SyncPlan, error) {
	roots := make([]domain.Root, 0, len(b.Roots))
	for root := range b.Roots {
		roots = append(roots, root)
	}
	slices.Sort(roots)
	plan := SyncPlan{Direction: domain.Restore, Selection: Selection{}}
	for _, root := range roots {
		src, dst := path.Join(b.Dir, string(root)), b.Roots[root]
		e := domain.ResolvedEntry{Entry: domain.Entry{Root: root}, RepoPath: src, DestPath: dst}
		items, err := s.planEntry(ctx, domain.Restore, e, src, dst, nil)
		if err != nil {
			return SyncPlan{}, err
		}
		for _, it := range items {
			if it.Change.Type == domain.Delete {
				continue
			}
			plan.Items = append(plan.Items, it)
			plan.Selection[it.Key()] = !it.Blocked
		}
	}
	return plan, nil
}

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
		if it.Change.Type != domain.Create && it.Change.Kind != domain.KindDir {
			files = append(files, it)
		}
	}
	return files
}

func (s *Service) Apply(ctx context.Context, plan SyncPlan, sel Selection, opts ApplyOptions, progress func(domain.StepProgress)) ApplyResult {
	items := selectedItems(plan, sel)
	if len(items) == 0 {
		return ApplyResult{Nothing: true}
	}
	var res ApplyResult
	applied := map[ItemKey]bool{}

	run := func(step domain.Step) (domain.StepResult, bool) {
		r := s.Runner.Run(ctx, step, progress)
		res.Outcomes = append(res.Outcomes, StepOutcome{Step: step, Result: r})
		if r.Status != domain.StepFailed {
			return r, true
		}
		if res.Err == nil {
			res.Err = r.Err
			if res.Err == nil {
				res.Err = errors.New(cmp.Or(r.Detail, "step failed"))
			}
		}
		return r, false
	}

	func() {
		if files := backupFiles(items); opts.Backup && len(files) > 0 {
			if _, ok := run(backupStep(plan.Direction, files)); !ok {
				return
			}
		}
		for _, g := range groupByEntry(items) {
			paths, group := copyPaths(g)
			if len(paths) == 0 {
				continue
			}
			r, ok := run(domain.CopyStep{Entry: g[0].Resolved, Direction: plan.Direction, Src: g[0].Src, Dst: g[0].Dst, Paths: paths, Dir: !g[0].fileEntry})
			for _, it := range group {
				if ok || slices.Contains(r.Applied, it.Change.Path) {
					applied[it.Key()] = true
				}
			}
			if !ok {
				return
			}
		}
		for _, it := range orderedDeletes(items) {
			if _, ok := run(domain.DeleteStep{Path: it.dstPath(), Root: it.Dst}); !ok {
				return
			}
			applied[it.Key()] = true
		}
		if plan.Direction != domain.Restore {
			if _, ok := run(domain.SaveStateStep{State: s.syncedState(ctx, plan.Direction)}); !ok {
				return
			}
		}
	}()

	for _, it := range items {
		result := ItemNotApplied
		if applied[it.Key()] {
			result = ItemApplied
		}
		res.Items = append(res.Items, ItemResult{Key: it.Key(), Result: result})
	}
	run(domain.AppendLogStep{Entry: s.logEntry(plan.Direction, opts.DryRun, items, res)})
	return res
}

func (s *Service) syncedState(ctx context.Context, dir domain.Direction) domain.State {
	// Re-read so ignores and state saved since planning are not overwritten.
	state, _ := s.Inspector.LoadState(ctx)
	state.RepoPath = s.RepoDir
	state.LastSync = &domain.LastSync{Time: s.Now(), Direction: directionName(dir)}
	return state
}

func (s *Service) logEntry(dir domain.Direction, dryRun bool, items []PlanItem, res ApplyResult) domain.LogEntry {
	entry := domain.LogEntry{
		Time:      s.Now().Format(time.RFC3339),
		Op:        "sync",
		Direction: directionName(dir),
		DryRun:    dryRun,
		Result:    "ok",
	}
	if dir == domain.Restore {
		entry.Op = "restore"
	}
	if res.Err != nil {
		entry.Result, entry.Error = "failed", res.Err.Error()
	}
	for i, it := range items {
		entry.Items = append(entry.Items, domain.LogItem{Path: it.rel(), Kind: kindName(it.Change.Kind), Result: res.Items[i].Result})
	}
	return entry
}

func directionName(d domain.Direction) string {
	switch d {
	case domain.Pull:
		return "pull"
	case domain.Restore:
		return "restore"
	}
	return "push"
}

func kindName(k domain.NodeKind) string {
	switch k {
	case domain.KindDir:
		return "dir"
	case domain.KindLink:
		return "link"
	}
	return "file"
}

// backupStep backs up files at the destination side; the roots are the destination root directories.
func backupStep(dir domain.Direction, files []PlanItem) domain.BackupStep {
	step := domain.BackupStep{Direction: dir, Roots: map[domain.Root]string{}}
	for _, it := range files {
		step.Files = append(step.Files, domain.BackupFile{Root: it.Change.Entry.Root, Path: it.rel()})
		step.Roots[it.Change.Entry.Root] = strings.TrimSuffix(it.Dst, "/"+it.Change.Entry.Path)
	}
	return step
}

func groupByEntry(items []PlanItem) [][]PlanItem {
	var groups [][]PlanItem
	index := map[domain.Entry]int{}
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

// copyPaths returns the paths to transfer and the items they cover.
func copyPaths(group []PlanItem) ([]string, []PlanItem) {
	var paths []string
	var copied []PlanItem
	for _, it := range group {
		if it.Change.Type != domain.Delete {
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
		case it.Change.Type != domain.Delete:
		case it.Change.Kind == domain.KindDir:
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

type DriftState int

const (
	DriftInSync DriftState = iota
	DriftDrifted
	DriftError
)

// DriftConcurrency bounds simultaneous entry diffs.
const DriftConcurrency = 4

type DriftResult struct {
	Entry      domain.ResolvedEntry
	State      DriftState
	Changes    int
	Err        error
	RepoTimes  domain.RepoTimes
	PCTime     time.Time
	LastBackup *domain.Backup
}

// Drift diffs repo to PC per entry and streams one result per entry; the channel closes when all are done.
func (s *Service) Drift(ctx context.Context, entries []domain.ResolvedEntry) <-chan DriftResult {
	out := make(chan DriftResult, len(entries))
	var lastBackup *domain.Backup
	backupDone := make(chan struct{})
	go func() {
		defer close(backupDone)
		lastBackup = s.lastCompleteBackup(ctx)
	}()
	slots := make(chan struct{}, DriftConcurrency)
	var wg sync.WaitGroup
	for _, e := range entries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
			}
			if err := ctx.Err(); err != nil {
				out <- DriftResult{Entry: e, State: DriftError, Err: err}
				return
			}
			r := s.driftEntry(ctx, e)
			<-backupDone
			r.LastBackup = lastBackup
			out <- r
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (s *Service) driftEntry(ctx context.Context, e domain.ResolvedEntry) DriftResult {
	r := DriftResult{Entry: e}
	changes, err := s.Inspector.Diff(ctx, e, domain.Endpoint{Path: e.RepoPath}, domain.Endpoint{Path: e.DestPath})
	if err != nil {
		r.State, r.Err = DriftError, err
		return r
	}
	if r.Changes = len(changes); r.Changes > 0 {
		r.State = DriftDrifted
	}
	r.RepoTimes, _ = s.Inspector.RepoTimes(ctx, s.RepoDir, string(e.Root)+"/"+e.Path)
	r.PCTime, _ = s.Inspector.NewestMTime(ctx, e.DestPath, e.Filter)
	return r
}

func (s *Service) lastCompleteBackup(ctx context.Context) *domain.Backup {
	backups, err := s.Inspector.ListBackups(ctx)
	if err != nil {
		return nil
	}
	for _, b := range backups {
		if !b.Incomplete && !b.Unreadable {
			return &b
		}
	}
	return nil
}

// DeleteBackup runs a DeleteBackupStep only when confirmed; the bool reports whether a step ran.
// ListBackups returns the existing backups newest first.
func (s *Service) ListBackups(ctx context.Context) ([]domain.Backup, error) {
	return s.Inspector.ListBackups(ctx)
}

func (s *Service) DeleteBackup(ctx context.Context, dir string, confirmed bool) (domain.StepResult, bool) {
	if !confirmed {
		return domain.StepResult{}, false
	}
	return s.Runner.Run(ctx, domain.DeleteBackupStep{Dir: dir}, nil), true
}
