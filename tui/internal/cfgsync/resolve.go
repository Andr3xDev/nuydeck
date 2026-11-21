package cfgsync

import (
	"io/fs"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// MaxEditBytes is the largest file the resolver reads or edits.
const MaxEditBytes = 1 << 20

// Side is what one end of an item holds, read for the resolver.
type Side struct {
	Exists bool
	Kind   plan.NodeKind
	Size   int64
	// Content is nil for a link, a binary file or a file above MaxEditBytes.
	Content    []byte
	Binary     bool
	TooLarge   bool
	LinkTarget string
	// LinkBroken is true for a link whose target does not resolve.
	LinkBroken bool
	// Children lists a directory's direct entries, sorted by name.
	Children []Child
	Mode     fs.FileMode
}

// Child is one direct entry of a directory side.
type Child struct {
	Name string
	Kind plan.NodeKind
	Size int64
}

// Edited is the content the user wrote for an item, applied instead of a copy.
type Edited struct {
	Content []byte
	Mode    fs.FileMode
}

// Resolutions maps an item to its edited content.
type Resolutions map[ItemKey]Edited

type Choice int

const (
	ChoiceKeep Choice = iota
	ChoiceTake
	ChoiceEdit
	ChoiceSkip
)

// ChoicesFor lists what the user may do with an item, given its destination side.
func ChoicesFor(it PlanItem, side Side) []Choice {
	switch {
	case it.Blocked, it.Change.Type == plan.Delete:
		return nil
	case it.Change.Type == plan.Create:
		return []Choice{ChoiceSkip, ChoiceTake}
	case it.Change.Kind == plan.KindLink, side.Kind == plan.KindLink, side.Binary, side.TooLarge:
		return []Choice{ChoiceKeep, ChoiceTake}
	}
	return []Choice{ChoiceKeep, ChoiceTake, ChoiceEdit}
}
