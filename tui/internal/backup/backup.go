package backup

import (
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Backup describes one folder under ~/.nuydeck-backups.
type Backup struct {
	Dir        string
	Created    time.Time
	Direction  plan.Direction
	Roots      map[manifest.Root]string
	Files      []string
	Incomplete bool
	Unreadable bool
}
