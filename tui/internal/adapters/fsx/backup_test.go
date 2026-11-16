package fsx_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/fsx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func requireRsync(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not installed")
	}
}

func fixedNow(ts time.Time) func() time.Time { return func() time.Time { return ts } }

func backupStep(dest string, rels ...string) plan.BackupStep {
	files := make([]plan.BackupFile, len(rels))
	for i, r := range rels {
		files[i] = plan.BackupFile{Root: manifest.RootConfig, Path: r}
	}
	return plan.BackupStep{Direction: plan.Push, Files: files, Roots: map[manifest.Root]string{manifest.RootConfig: dest}}
}

func TestBackupStep_WritesCompleteBackupWithoutPartial(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	dest := filepath.Join(home, ".config")
	writeFile(t, filepath.Join(dest, "nvim", "init.lua"), "old")
	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	res := fsx.Runner{Home: home, Now: fixedNow(ts)}.Run(context.Background(), backupStep(dest, "nvim/init.lua"), nil)

	if res.Status != plan.StepOK {
		t.Fatalf("status = %v, err = %v", res.Status, res.Err)
	}
	final := filepath.Join(fsx.BackupsDir(home), "20260301-100000")
	if b, err := os.ReadFile(filepath.Join(final, "config", "nvim", "init.lua")); err != nil || string(b) != "old" {
		t.Fatalf("backed up file = %q, %v", b, err)
	}
	if !exists(filepath.Join(final, "meta.toml")) {
		t.Fatal("meta.toml missing")
	}
	if exists(final + ".partial") {
		t.Fatal(".partial left behind after successful backup")
	}
	backups, err := fsx.ListBackups(context.Background(), home)
	if err != nil || len(backups) != 1 {
		t.Fatalf("ListBackups = %v, %v", backups, err)
	}
	got := backups[0]
	if got.Incomplete || got.Unreadable || got.Direction != plan.Push || got.Roots[manifest.RootConfig] != dest ||
		!slices.Equal(got.Files, []string{"config/nvim/init.lua"}) || !got.Created.Equal(ts) {
		t.Fatalf("meta round trip mismatch: %+v", got)
	}
}

func TestBackupStep_SameTimestampGetsCollisionSuffix(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	dest := filepath.Join(home, ".config")
	writeFile(t, filepath.Join(dest, "a"), "1")
	r := fsx.Runner{Home: home, Now: fixedNow(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC))}

	for range 2 {
		if res := r.Run(context.Background(), backupStep(dest, "a"), nil); res.Status != plan.StepOK {
			t.Fatalf("status = %v, err = %v", res.Status, res.Err)
		}
	}

	for _, name := range []string{"20260301-100000", "20260301-100000-1"} {
		if !exists(filepath.Join(fsx.BackupsDir(home), name, "meta.toml")) {
			t.Fatalf("backup %s missing", name)
		}
	}
}

func TestBackupStep_FailureReturnsErrorAndNoCompleteBackup(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	dest := filepath.Join(home, ".config")
	_ = os.MkdirAll(dest, 0o755)

	res := fsx.Runner{Home: home, Now: fixedNow(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC))}.
		Run(context.Background(), backupStep(dest, "does-not-exist"), nil)

	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("status = %v, err = %v; want failure", res.Status, res.Err)
	}
	entries, _ := os.ReadDir(fsx.BackupsDir(home))
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".partial") {
			t.Fatalf("complete-looking backup %q exists after failure", e.Name())
		}
	}
}

func TestBackupStep_RefusedWhenBackupsDirInsideDestinationEntry(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "keep.txt"), "x")
	step := plan.BackupStep{
		Files: []plan.BackupFile{{Root: manifest.RootHome, Path: ".nuydeck-backups"}},
		Roots: map[manifest.Root]string{manifest.RootHome: home},
	}
	_ = os.MkdirAll(fsx.BackupsDir(home), 0o755)

	res := fsx.Runner{Home: home}.Run(context.Background(), step, nil)

	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("status = %v, err = %v; want refusal", res.Status, res.Err)
	}
	if entries, _ := os.ReadDir(fsx.BackupsDir(home)); len(entries) != 0 {
		t.Fatalf("refused backup still created %d entries", len(entries))
	}
}

func TestBackupStep_RefusedWhenBackupsDirReachedThroughSymlinkedRoot(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	_ = os.MkdirAll(fsx.BackupsDir(home), 0o755)
	linkedRoot := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(home, linkedRoot); err != nil {
		t.Fatal(err)
	}
	step := plan.BackupStep{
		Files: []plan.BackupFile{{Root: manifest.RootHome, Path: ".nuydeck-backups"}},
		Roots: map[manifest.Root]string{manifest.RootHome: linkedRoot},
	}

	res := fsx.Runner{Home: home}.Run(context.Background(), step, nil)

	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("status = %v, err = %v; want refusal", res.Status, res.Err)
	}
	if entries, _ := os.ReadDir(fsx.BackupsDir(home)); len(entries) != 0 {
		t.Fatalf("refused backup still created %d entries", len(entries))
	}
}

func TestBackupStep_RefusedWhenEntryLiesInsideBackupsDir(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	backups := fsx.BackupsDir(home)
	writeFile(t, filepath.Join(backups, "20260101-000000", "f"), "x")
	step := plan.BackupStep{
		Files: []plan.BackupFile{{Root: manifest.RootHome, Path: "20260101-000000/f"}},
		Roots: map[manifest.Root]string{manifest.RootHome: backups},
	}

	res := fsx.Runner{Home: home}.Run(context.Background(), step, nil)

	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("status = %v, err = %v; want refusal", res.Status, res.Err)
	}
	if entries, _ := os.ReadDir(backups); len(entries) != 1 {
		t.Fatalf("refused backup changed the backups dir: %d entries", len(entries))
	}
}

func TestListBackups_NewestFirstAndFlagsBrokenOnes(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	dest := filepath.Join(home, ".config")
	writeFile(t, filepath.Join(dest, "a"), "1")
	for _, ts := range []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	} {
		if res := (fsx.Runner{Home: home, Now: fixedNow(ts)}).Run(context.Background(), backupStep(dest, "a"), nil); res.Status != plan.StepOK {
			t.Fatalf("backup: %v", res.Err)
		}
	}
	root := fsx.BackupsDir(home)
	_ = os.MkdirAll(filepath.Join(root, "20260401-000000.partial"), 0o755)
	writeFile(t, filepath.Join(root, "20260501-000000", "meta.toml"), "not = [valid")

	got, err := fsx.ListBackups(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}

	var complete []time.Time
	flagged := map[string]domain.Backup{}
	for _, b := range got {
		if b.Incomplete || b.Unreadable {
			flagged[filepath.Base(b.Dir)] = b
			continue
		}
		complete = append(complete, b.Created)
	}
	if len(complete) != 3 || !complete[0].After(complete[1]) || !complete[1].After(complete[2]) {
		t.Fatalf("complete backups not newest first: %v", complete)
	}
	if !flagged["20260401-000000.partial"].Incomplete {
		t.Fatalf(".partial not flagged incomplete: %+v", flagged)
	}
	if !flagged["20260501-000000"].Unreadable {
		t.Fatalf("bad meta.toml not flagged unreadable: %+v", flagged)
	}
}

func TestListBackups_MissingDirIsEmptyNotError(t *testing.T) {
	got, err := fsx.ListBackups(context.Background(), t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDeleteBackupStep(t *testing.T) {
	home := t.TempDir()
	root := fsx.BackupsDir(home)
	inside := filepath.Join(root, "20260301-100000")
	writeFile(t, filepath.Join(inside, "meta.toml"), "x")
	elsewhere := filepath.Join(home, "precious")
	writeFile(t, filepath.Join(elsewhere, "f"), "x")
	r := fsx.Runner{Home: home}
	del := func(dir string) plan.StepResult {
		return r.Run(context.Background(), plan.DeleteBackupStep{Dir: dir}, nil)
	}

	for name, dir := range map[string]string{
		"outside backups root": elsewhere,
		"backups root itself":  root,
		"dot-dot traversal":    filepath.Join(root, "..", "precious"),
		"deeper descendant":    filepath.Join(inside, "meta.toml"),
	} {
		if res := del(dir); res.Status != plan.StepFailed || res.Err == nil {
			t.Errorf("%s: status = %v, err = %v; want refusal", name, res.Status, res.Err)
		}
	}
	if !exists(elsewhere) || !exists(inside) {
		t.Fatal("refused deletes removed something")
	}

	if res := del(inside); res.Status != plan.StepOK {
		t.Fatalf("delete inside: %v", res.Err)
	}
	if exists(inside) {
		t.Fatal("backup folder still exists")
	}
}
