package state

import "time"

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
