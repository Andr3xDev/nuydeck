package app

import "github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"

// Service keeps the drift use case next to the sync service until they leave app.
type Service struct {
	*cfgsync.Service
	Inspector Inspector
}
