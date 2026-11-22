package envvars

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

// ErrNoHyprEntry means the manifest has no config/hypr entry to locate the env file from.
var ErrNoHyprEntry = errors.New("no hypr entry in the manifest")

// envFile is the only file this package ever builds a path to.
const envFile = "configs/env_variables.lua"

// Source picks which copy of the env file is read or edited.
type Source int

const (
	SourceRepo Source = iota
	SourceLive
)

// Locations holds both copies of the env file.
type Locations struct {
	RepoPath string
	DestPath string
	// LiveRoot and LiveRel split DestPath into the config root and the file path under it, as a backup needs.
	LiveRoot string
	LiveRel  string
}

// Path returns the copy chosen by source.
func (l Locations) Path(source Source) string {
	if source == SourceLive {
		return l.DestPath
	}
	return l.RepoPath
}

// Paths derives both copies of the env file from the resolved config/hypr entry.
func Paths(entries []manifest.ResolvedEntry) (Locations, error) {
	for _, e := range entries {
		if e.Root == manifest.RootConfig && e.Path == "hypr" {
			return Locations{
				RepoPath: filepath.Join(e.RepoPath, envFile),
				DestPath: filepath.Join(e.DestPath, envFile),
				LiveRoot: strings.TrimSuffix(e.DestPath, "/"+e.Path),
				LiveRel:  filepath.Join(e.Path, envFile),
			}, nil
		}
	}
	return Locations{}, ErrNoHyprEntry
}
