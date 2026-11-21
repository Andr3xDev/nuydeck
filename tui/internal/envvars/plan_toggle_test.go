package envvars_test

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/envvars"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const toggleSource = "hl.env(\"A\", \"1\")\nhl.env(\"B\", \"2\")\n"

func toggleLocations(t *testing.T) envvars.Locations {
	t.Helper()
	loc, err := envvars.Paths([]manifest.ResolvedEntry{hyprEntry()})
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestPlanToggle_LiveCopyIsBackedUpBeforeTheWrite(t *testing.T) {
	loc := toggleLocations(t)
	v := varAt(t, toggleSource, 1)

	got, change, err := envvars.PlanToggle(envvars.SourceLive, loc, []byte(toggleSource), 0o640, v)

	if err != nil {
		t.Fatalf("PlanToggle() error = %v", err)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("steps = %+v, want backup then write", got.Steps)
	}
	backup, ok := got.Steps[0].(plan.BackupStep)
	if !ok {
		t.Fatalf("step 0 = %T, want plan.BackupStep", got.Steps[0])
	}
	if backup.Direction != plan.Manual {
		t.Errorf("backup direction = %v, want Manual", backup.Direction)
	}
	if backup.Roots[manifest.RootConfig] != loc.LiveRoot || len(backup.Roots) != 1 {
		t.Errorf("backup roots = %v, want only config -> %q", backup.Roots, loc.LiveRoot)
	}
	wantFiles := []plan.BackupFile{{Root: manifest.RootConfig, Path: loc.LiveRel}}
	if !slices.Equal(backup.Files, wantFiles) {
		t.Errorf("backup files = %+v, want %+v", backup.Files, wantFiles)
	}
	write, ok := got.Steps[1].(plan.WriteFileStep)
	if !ok {
		t.Fatalf("step 1 = %T, want plan.WriteFileStep", got.Steps[1])
	}
	if write.Path != loc.DestPath {
		t.Errorf("write path = %q, want the live copy %q", write.Path, loc.DestPath)
	}
	if want := "hl.env(\"A\", \"1\")\n-- hl.env(\"B\", \"2\")\n"; string(write.Content) != want {
		t.Errorf("write content = %q, want %q", write.Content, want)
	}
	if write.Mode != 0o640 {
		t.Errorf("write mode = %v, want 0640", write.Mode)
	}
	if write.Root != filepath.Dir(loc.DestPath) || write.Privileged {
		t.Errorf("write confinement = root %q privileged %v, want unprivileged inside %q", write.Root, write.Privileged, filepath.Dir(loc.DestPath))
	}
	if change.Before != `hl.env("B", "2")` || change.After != `-- hl.env("B", "2")` {
		t.Errorf("change = %+v", change)
	}
}

func TestPlanToggle_RepoCopyHasNoBackupStep(t *testing.T) {
	loc := toggleLocations(t)

	got, _, err := envvars.PlanToggle(envvars.SourceRepo, loc, []byte(toggleSource), 0o644, varAt(t, toggleSource, 0))

	if err != nil {
		t.Fatalf("PlanToggle() error = %v", err)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("steps = %+v, want only the write", got.Steps)
	}
	write, ok := got.Steps[0].(plan.WriteFileStep)
	if !ok {
		t.Fatalf("step 0 = %T, want plan.WriteFileStep", got.Steps[0])
	}
	if write.Path != loc.RepoPath || write.Root != filepath.Dir(loc.RepoPath) {
		t.Errorf("write = path %q root %q, want the repo copy %q", write.Path, write.Root, loc.RepoPath)
	}
}

func TestPlanToggle_WritesNothingButTheEnvFile(t *testing.T) {
	loc := toggleLocations(t)
	for _, source := range []envvars.Source{envvars.SourceRepo, envvars.SourceLive} {
		got, _, err := envvars.PlanToggle(source, loc, []byte(toggleSource), 0o644, varAt(t, toggleSource, 0))
		if err != nil {
			t.Fatal(err)
		}
		var written []string
		for _, s := range got.Steps {
			switch s := s.(type) {
			case plan.WriteFileStep:
				written = append(written, s.Path)
			case plan.BackupStep:
			default:
				t.Errorf("source %v: unexpected step %T", source, s)
			}
		}
		if !slices.Equal(written, []string{loc.Path(source)}) {
			t.Errorf("source %v: written paths = %v, want only %q", source, written, loc.Path(source))
		}
	}
}

func TestPlanToggle_StaleRowReturnsErrLineChangedAndNoPlan(t *testing.T) {
	stale := envvars.Var{Name: "A", Value: "1", Line: 1}

	got, _, err := envvars.PlanToggle(envvars.SourceLive, toggleLocations(t), []byte(toggleSource), 0o644, stale)

	if !errors.Is(err, envvars.ErrLineChanged) {
		t.Fatalf("PlanToggle() error = %v, want ErrLineChanged", err)
	}
	if len(got.Steps) != 0 {
		t.Errorf("steps = %+v, want none on error", got.Steps)
	}
}
