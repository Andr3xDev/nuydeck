package domain

import "time"

type Direction int

const (
	Push Direction = iota
	Pull
	Restore
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
	Entry   Entry
	Path    string // relative to the entry root
	Type    ChangeType
	Kind    NodeKind
	Warning LinkWarning
}

// Plan is the computed difference for one direction.
type Plan struct {
	Direction Direction
	Changes   []Change
}

// RepoTimes holds last-updated times read from git.
type RepoTimes struct {
	Entry   time.Time
	Files   map[string]time.Time
	Unknown bool
}

// Backup describes one folder under ~/.nuydeck-backups.
type Backup struct {
	Dir        string
	Created    time.Time
	Direction  Direction
	Roots      map[Root]string
	Files      []string
	Incomplete bool
	Unreadable bool
}

// State is the content of the local state.toml.
type State struct {
	Version  int       `toml:"version"`
	RepoPath string    `toml:"repo_path"`
	LastSync *LastSync `toml:"last_sync"`
	Ignores  []string  `toml:"ignores"`

	IgnoredPackages []string `toml:"ignored_packages,omitempty"`
}

type LastSync struct {
	Time      time.Time `toml:"time"`
	Direction string    `toml:"direction"`
}
