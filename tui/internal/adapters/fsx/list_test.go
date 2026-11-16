package fsx_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/fsx"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

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
