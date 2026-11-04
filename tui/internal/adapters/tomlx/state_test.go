package tomlx_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/tomlx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func TestStateAndLogLocations(t *testing.T) {
	if got, want := tomlx.StatePath("/h"), "/h/.local/state/nuydeck/state.toml"; got != want {
		t.Errorf("StatePath = %q, want %q", got, want)
	}
	if got, want := tomlx.LogsDir("/c"), "/c/nuydeck/logs"; got != want {
		t.Errorf("LogsDir = %q, want %q", got, want)
	}
}

func TestState_SaveThenLoadRoundTripCreatingMissingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "nuydeck", "state.toml")
	want := domain.State{
		Version:  1,
		RepoPath: "/home/u/nuydeck",
		LastSync: &domain.LastSync{Time: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), Direction: "push"},
		Ignores:  []string{"config:nvim/lazy-lock.json"},
	}

	if err := tomlx.SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	got, warning := tomlx.LoadState(path)

	if warning != "" || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v (warning %q), want %+v", got, warning, want)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("dir should hold only state.toml, has %d entries", len(entries))
	}
}

func TestState_FailedSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.toml")
	if err := os.Mkdir(path, 0o755); err != nil { // rename onto a directory fails
		t.Fatal(err)
	}

	if err := tomlx.SaveState(path, domain.State{Version: 1}); err == nil {
		t.Fatal("expected error")
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestState_MissingCorruptOrUnknownVersionGivesDefaultsAndWarning(t *testing.T) {
	cases := map[string]*string{
		"missing":         nil,
		"corrupt":         ptr("repo_path = [unterminated"),
		"unknown version": ptr("version = 99\nrepo_path = \"/x\"\n"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.toml")
			if content != nil {
				if err := os.WriteFile(path, []byte(*content), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			got, warning := tomlx.LoadState(path)

			if warning == "" {
				t.Error("expected a warning")
			}
			if got.Version != 1 || got.RepoPath != "" || got.LastSync != nil || len(got.Ignores) != 0 {
				t.Errorf("not defaults: %+v", got)
			}
			if content != nil {
				if b, _ := os.ReadFile(path); string(b) != *content {
					t.Errorf("file modified on load: %q", b)
				}
			}
		})
	}
}

func TestState_SaveInsideRepoIsRefused(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, ".state", "state.toml")

	err := tomlx.SaveState(path, domain.State{Version: 1, RepoPath: repo})

	if err == nil {
		t.Fatal("expected refusal")
	}
	if _, statErr := os.Stat(filepath.Dir(path)); statErr == nil {
		t.Fatal("refused save still created files in the repo")
	}
}

func TestState_SaveThroughSymlinkIntoRepoIsRefused(t *testing.T) {
	repo, outside := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(outside, "state")
	if err := os.Symlink(filepath.Join(repo, "inner"), link); err != nil {
		t.Fatal(err)
	}

	err := tomlx.SaveState(filepath.Join(link, "state.toml"), domain.State{Version: 1, RepoPath: repo})

	if err == nil {
		t.Fatal("expected refusal: state dir resolves inside the repo")
	}
	if _, statErr := os.Stat(filepath.Join(repo, "inner", "state.toml")); statErr == nil {
		t.Fatal("state was written inside the repo")
	}
}

func TestState_SaveRefusedWhenContainmentCannotBeDetermined(t *testing.T) {
	t.Chdir(t.TempDir())

	err := tomlx.SaveState("state.toml", domain.State{Version: 1, RepoPath: t.TempDir()})

	if err == nil {
		t.Fatal("expected refusal: relative path vs absolute repo is undeterminable")
	}
	if _, statErr := os.Stat("state.toml"); statErr == nil {
		t.Fatal("state was written despite undeterminable containment")
	}
}

func TestAppendLog_CreatesDirAppendsJSONLByMonthWithOnlyKnownFields(t *testing.T) {
	logs := filepath.Join(t.TempDir(), "nuydeck", "logs")
	entry := func(ts, result string) domain.LogEntry {
		return domain.LogEntry{
			Time: ts, Op: "sync", Direction: "push", Result: result,
			Items: []domain.LogItem{{Path: "config:nvim/init.lua", Kind: "file", Result: "ok"}},
		}
	}

	for _, e := range []domain.LogEntry{entry("2026-03-15T10:00:00Z", "ok"), entry("2026-03-16T11:00:00Z", "failed")} {
		if err := tomlx.AppendLog(logs, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := tomlx.AppendLog(logs, entry("2026-04-01T00:00:00Z", "ok")); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(logs, "2026-03.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines in 2026-03.jsonl, got %d", len(lines))
	}
	allowed := map[string]bool{"ts": true, "op": true, "direction": true, "dry_run": true, "items": true, "result": true, "error": true}
	for _, line := range lines {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		for k := range obj {
			if !allowed[k] {
				t.Errorf("unexpected log field %q", k)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(logs, "2026-04.jsonl")); err != nil {
		t.Fatal("April entry not in its own file")
	}
}

func ptr(s string) *string { return &s }
