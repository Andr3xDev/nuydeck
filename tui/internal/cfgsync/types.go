package cfgsync

import (
	"path"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// ItemKey identifies a plan item: the managed root and the root-relative path.
type ItemKey struct {
	Root manifest.Root
	Path string
}

// Selection holds the keys of the items to apply.
type Selection map[ItemKey]bool

// PlanItem is one offered change with everything the preview shows.
// Src and Dst are the endpoints of the entry. For a file entry they are the
// file paths and Change.Path is the file's base name, as rsync itemizes it.
type PlanItem struct {
	Change   plan.Change
	Resolved manifest.ResolvedEntry
	Src, Dst string
	Blocked  bool
	// Ignored marks an item the user ignores; it is offered but not selected by default.
	Ignored bool
	// RepoTime and PCTime are zero for the side that does not have the file.
	RepoTime, PCTime time.Time
	FileEntry        bool
}

// rel is the root-relative path of the item.
func (i PlanItem) rel() string {
	if i.FileEntry {
		return i.Change.Entry.Path
	}
	return path.Join(i.Change.Entry.Path, i.Change.Path)
}

func (i PlanItem) SrcPath() string { return i.endpoint(i.Src) }
func (i PlanItem) DstPath() string { return i.endpoint(i.Dst) }

func (i PlanItem) endpoint(entryPath string) string {
	if i.FileEntry {
		return entryPath
	}
	return path.Join(entryPath, i.Change.Path)
}

// Key is the root-relative identity of the item (entry path joined with the change path).
func (i PlanItem) Key() ItemKey { return ItemKey{Root: i.Change.Entry.Root, Path: i.rel()} }

type SyncPlan struct {
	Direction    plan.Direction
	Items        []PlanItem
	Selection    Selection
	StateWarning string
	// Notices lists entries left out of the plan, e.g. a missing source.
	Notices []string
	// Resolutions holds the edited content per item; entries of deselected items are ignored.
	Resolutions Resolutions
}

type ApplyOptions struct {
	// Backup is true when the user accepted the backup offer.
	Backup bool
	DryRun bool
	// Hard makes Apply delete the selected Delete items; a normal run never deletes.
	Hard bool
	// Mandatory marks a backup that is taken without being offered.
	Mandatory bool
}

type StepOutcome struct {
	Step   plan.Step
	Result plan.StepResult
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
	// Notices lists things the owner should know that are not failures, e.g. a skipped state save.
	Notices []string
	Err     error
}
