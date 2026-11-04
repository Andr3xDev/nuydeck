package tomlx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/fsx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const (
	stateVersion   = 1
	logMonthLayout = "2006-01"
)

// StatePath is <home>/.local/state/nuydeck/state.toml.
func StatePath(home string) string {
	return filepath.Join(home, ".local", "state", "nuydeck", "state.toml")
}

// LogsDir is <configDir>/nuydeck/logs.
func LogsDir(configDir string) string { return filepath.Join(configDir, "nuydeck", "logs") }

// LoadState reads the state file; a missing, corrupt or unsupported file yields
// defaults and a warning, and is left untouched until the next SaveState.
func LoadState(path string) (state domain.State, warning string) {
	defaults := domain.State{Version: stateVersion}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaults, fmt.Sprintf("state not loaded (%v); using defaults", err)
	}
	if err := toml.Unmarshal(data, &state); err != nil {
		return defaults, fmt.Sprintf("state file is corrupt (%v); using defaults", err)
	}
	if state.Version != stateVersion {
		return defaults, fmt.Sprintf("state file version %d is not supported; using defaults", state.Version)
	}
	return state, ""
}

// SaveState writes the state atomically and refuses a path inside the repo.
// Containment is checked on symlink-resolved paths and fails closed.
func SaveState(path string, s domain.State) error {
	dir := filepath.Dir(path)
	if s.RepoPath != "" {
		if err := refuseInsideRepo(dir, s.RepoPath); err != nil {
			return err
		}
	}
	s.Version = stateVersion
	data, err := toml.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
	}
	return werr
}

func refuseInsideRepo(dir, repo string) error {
	realDir, err := fsx.ResolveExisting(dir)
	if err != nil {
		return err
	}
	realRepo, err := fsx.ResolveExisting(repo)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realRepo, realDir)
	if err != nil {
		return fmt.Errorf("cannot determine whether state dir %s is inside the repo: %w", dir, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	return fmt.Errorf("refusing to write state inside the repo: %s", dir)
}

// AppendLog appends e as one JSON line to <logsDir>/YYYY-MM.jsonl, the month taken from e.Time.
func AppendLog(logsDir string, e domain.LogEntry) error {
	ts, err := time.Parse(time.RFC3339, e.Time)
	if err != nil {
		return fmt.Errorf("log entry time: %w", err)
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(logsDir, ts.UTC().Format(logMonthLayout)+".jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}
