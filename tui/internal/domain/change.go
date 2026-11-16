package domain

import (
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

// NodeKind is the lstat classification of a path.
type NodeKind int

const (
	KindNone NodeKind = iota
	KindFile
	KindDir
	KindLink
)

type ChangeType int

const (
	Create ChangeType = iota
	Update
	Delete
)

type LinkWarning int

const (
	NoLinkWarning LinkWarning = iota
	WarnAbsoluteLink
	WarnLeavesManagedSet
)

// Endpoint is one side of a diff: an absolute root directory or file path.
type Endpoint struct {
	Path string
}

// Change is one itemized difference for an entry.
type Change struct {
	Entry   manifest.Entry
	Path    string // relative to the entry root
	Type    ChangeType
	Kind    NodeKind
	Warning LinkWarning
}

// RepoTimes holds last-updated times read from git.
type RepoTimes struct {
	Entry   time.Time
	Files   map[string]time.Time
	Unknown bool
}
