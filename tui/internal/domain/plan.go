package domain

import (
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

// Plan is the computed difference for one direction.
type Plan struct {
	Direction Direction
	Changes   []Change
}

// Backup describes one folder under ~/.nuydeck-backups.
type Backup struct {
	Dir        string
	Created    time.Time
	Direction  Direction
	Roots      map[manifest.Root]string
	Files      []string
	Incomplete bool
	Unreadable bool
}
