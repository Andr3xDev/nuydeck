package backup

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func resolved(root manifest.Root, path, dest string) manifest.ResolvedEntry {
	return manifest.ResolvedEntry{
		Entry:    manifest.Entry{Root: root, Path: path},
		RepoPath: "/repo/" + string(root) + "/" + path,
		DestPath: dest,
	}
}

func TestPlanManual_BacksUpExistingEntriesAndSkipsMissingOnes(t *testing.T) {
	hypr := resolved(manifest.RootConfig, "hypr", "/home/u/.config/hypr")
	waybar := resolved(manifest.RootConfig, "waybar", "/home/u/.config/waybar")
	zshrc := resolved(manifest.RootHome, ".zshrc", "/home/u/.zshrc")
	in := &fakeInspector{kinds: map[string]plan.NodeKind{
		hypr.DestPath:  plan.KindDir,
		zshrc.DestPath: plan.KindFile,
	}}

	got, err := newService(t, in, &fakeRunner{}).PlanManual(context.Background(), []manifest.ResolvedEntry{hypr, waybar, zshrc})

	if err != nil {
		t.Fatal(err)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("steps = %+v, want one backup step", got.Steps)
	}
	step, ok := got.Steps[0].(plan.BackupStep)
	if !ok {
		t.Fatalf("step = %T, want plan.BackupStep", got.Steps[0])
	}
	if step.Direction != plan.Manual {
		t.Errorf("direction = %v, want Manual", step.Direction)
	}
	wantFiles := []plan.BackupFile{{Root: manifest.RootConfig, Path: "hypr"}, {Root: manifest.RootHome, Path: ".zshrc"}}
	if !slices.Equal(step.Files, wantFiles) {
		t.Errorf("files = %+v, want %+v (waybar is absent and skipped)", step.Files, wantFiles)
	}
	if step.Roots[manifest.RootConfig] != "/home/u/.config" || step.Roots[manifest.RootHome] != "/home/u" {
		t.Errorf("roots = %v, want the destination root directories", step.Roots)
	}
	if !slices.Contains(got.Notices, "Manifest filters are not applied, excluded files are included.") {
		t.Errorf("notices = %q, want the filters notice", got.Notices)
	}
}

func TestPlanManual_NothingToBackUpReturnsErrNothingToBackUp(t *testing.T) {
	in := &fakeInspector{kinds: map[string]plan.NodeKind{}}
	entries := []manifest.ResolvedEntry{resolved(manifest.RootConfig, "hypr", "/home/u/.config/hypr")}

	got, err := newService(t, in, &fakeRunner{}).PlanManual(context.Background(), entries)

	if !errors.Is(err, ErrNothingToBackUp) {
		t.Fatalf("err = %v, want ErrNothingToBackUp", err)
	}
	if len(got.Steps) != 0 {
		t.Errorf("steps = %+v, want none", got.Steps)
	}
}
