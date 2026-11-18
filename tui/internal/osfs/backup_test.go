package osfs_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
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

	res := osfs.Runner{Home: home, Now: fixedNow(ts)}.Run(context.Background(), backupStep(dest, "nvim/init.lua"), nil)

	if res.Status != plan.StepOK {
		t.Fatalf("status = %v, err = %v", res.Status, res.Err)
	}
	final := filepath.Join(backup.BackupsDir(home), "20260301-100000")
	if b, err := os.ReadFile(filepath.Join(final, "config", "nvim", "init.lua")); err != nil || string(b) != "old" {
		t.Fatalf("backed up file = %q, %v", b, err)
	}
	if !exists(filepath.Join(final, "meta.toml")) {
		t.Fatal("meta.toml missing")
	}
	if exists(final + ".partial") {
		t.Fatal(".partial left behind after successful backup")
	}
	backups, err := backup.ListBackups(context.Background(), home)
	if err != nil || len(backups) != 1 {
		t.Fatalf("ListBackups = %v, %v", backups, err)
	}
	got := backups[0]
	if got.Incomplete || got.Unreadable || got.Direction != plan.Push || got.Roots[manifest.RootConfig] != dest ||
		!slices.Equal(got.Files, []string{"config/nvim/init.lua"}) || !got.Created.Equal(ts) {
		t.Fatalf("meta round trip mismatch: %+v", got)
	}
}

func TestBackupStep_DanglingLinkAtDestinationIsBackedUpAsALink(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	dest := filepath.Join(home, ".config")
	if err := os.MkdirAll(filepath.Join(dest, "nvim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(dest, "nvim", "init.lua")); err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	res := osfs.Runner{Home: home, Now: fixedNow(ts)}.Run(context.Background(), backupStep(dest, "nvim/init.lua"), nil)

	if res.Status != plan.StepOK {
		t.Fatalf("status = %v, err = %v", res.Status, res.Err)
	}
	saved := filepath.Join(backup.BackupsDir(home), "20260301-100000", "config", "nvim", "init.lua")
	if got, err := os.Readlink(saved); err != nil || got != filepath.Join(home, "missing") {
		t.Fatalf("backed up link = %q, %v", got, err)
	}
}

func TestBackupStep_SameTimestampGetsCollisionSuffix(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	dest := filepath.Join(home, ".config")
	writeFile(t, filepath.Join(dest, "a"), "1")
	r := osfs.Runner{Home: home, Now: fixedNow(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC))}

	for range 2 {
		if res := r.Run(context.Background(), backupStep(dest, "a"), nil); res.Status != plan.StepOK {
			t.Fatalf("status = %v, err = %v", res.Status, res.Err)
		}
	}

	for _, name := range []string{"20260301-100000", "20260301-100000-1"} {
		if !exists(filepath.Join(backup.BackupsDir(home), name, "meta.toml")) {
			t.Fatalf("backup %s missing", name)
		}
	}
}

func TestBackupStep_FailureReturnsErrorAndNoCompleteBackup(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	dest := filepath.Join(home, ".config")
	_ = os.MkdirAll(dest, 0o755)

	res := osfs.Runner{Home: home, Now: fixedNow(time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC))}.
		Run(context.Background(), backupStep(dest, "does-not-exist"), nil)

	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("status = %v, err = %v; want failure", res.Status, res.Err)
	}
	entries, _ := os.ReadDir(backup.BackupsDir(home))
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
	_ = os.MkdirAll(backup.BackupsDir(home), 0o755)

	res := osfs.Runner{Home: home}.Run(context.Background(), step, nil)

	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("status = %v, err = %v; want refusal", res.Status, res.Err)
	}
	if entries, _ := os.ReadDir(backup.BackupsDir(home)); len(entries) != 0 {
		t.Fatalf("refused backup still created %d entries", len(entries))
	}
}

func TestBackupStep_RefusedWhenBackupsDirReachedThroughSymlinkedRoot(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	_ = os.MkdirAll(backup.BackupsDir(home), 0o755)
	linkedRoot := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(home, linkedRoot); err != nil {
		t.Fatal(err)
	}
	step := plan.BackupStep{
		Files: []plan.BackupFile{{Root: manifest.RootHome, Path: ".nuydeck-backups"}},
		Roots: map[manifest.Root]string{manifest.RootHome: linkedRoot},
	}

	res := osfs.Runner{Home: home}.Run(context.Background(), step, nil)

	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("status = %v, err = %v; want refusal", res.Status, res.Err)
	}
	if entries, _ := os.ReadDir(backup.BackupsDir(home)); len(entries) != 0 {
		t.Fatalf("refused backup still created %d entries", len(entries))
	}
}

func TestBackupStep_RefusedWhenEntryLiesInsideBackupsDir(t *testing.T) {
	requireRsync(t)
	home := t.TempDir()
	backups := backup.BackupsDir(home)
	writeFile(t, filepath.Join(backups, "20260101-000000", "f"), "x")
	step := plan.BackupStep{
		Files: []plan.BackupFile{{Root: manifest.RootHome, Path: "20260101-000000/f"}},
		Roots: map[manifest.Root]string{manifest.RootHome: backups},
	}

	res := osfs.Runner{Home: home}.Run(context.Background(), step, nil)

	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("status = %v, err = %v; want refusal", res.Status, res.Err)
	}
	if entries, _ := os.ReadDir(backups); len(entries) != 1 {
		t.Fatalf("refused backup changed the backups dir: %d entries", len(entries))
	}
}

func TestDeleteBackupStep(t *testing.T) {
	home := t.TempDir()
	root := backup.BackupsDir(home)
	inside := filepath.Join(root, "20260301-100000")
	writeFile(t, filepath.Join(inside, "meta.toml"), "x")
	elsewhere := filepath.Join(home, "precious")
	writeFile(t, filepath.Join(elsewhere, "f"), "x")
	r := osfs.Runner{Home: home}
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
