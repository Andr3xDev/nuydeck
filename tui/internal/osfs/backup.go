package osfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/fsutil"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/rsync"
)

const backupTimeLayout = "20060102-150405"

func (r Runner) backup(ctx context.Context, s plan.BackupStep) plan.StepResult {
	backupsDir := backup.BackupsDir(r.Home)
	realBackupsDir, err := fsutil.ResolveExisting(backupsDir)
	if err != nil {
		return failed(err)
	}
	for _, f := range s.Files {
		realEntry, err := fsutil.ResolveExisting(filepath.Join(s.Roots[f.Root], f.Path))
		if err != nil {
			return failed(err)
		}
		if fsutil.Within(realEntry, realBackupsDir) || fsutil.Within(realBackupsDir, realEntry) {
			return failed(fmt.Errorf("refusing backup: %s lies inside %s/%s", backupsDir, f.Root, f.Path))
		}
	}
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return failed(err)
	}
	now := r.now().UTC()
	final, partial, err := reserveBackupDir(backupsDir, now.Format(backupTimeLayout))
	if err != nil {
		return failed(err)
	}
	if err := writeBackup(ctx, s, partial, now); err != nil {
		_ = os.RemoveAll(partial)
		return failed(err)
	}
	if err := os.Rename(partial, final); err != nil {
		_ = os.RemoveAll(partial)
		return failed(err)
	}
	return plan.StepResult{Status: plan.StepOK, Detail: final}
}

// reserveBackupDir creates the .partial folder for the first free name among
// ts, ts-1, ts-2, ...
func reserveBackupDir(backupsDir, ts string) (final, partial string, err error) {
	for i := 0; ; i++ {
		name := ts
		if i > 0 {
			name = fmt.Sprintf("%s-%d", ts, i)
		}
		final, partial = filepath.Join(backupsDir, name), filepath.Join(backupsDir, name+backup.PartialSuffix)
		if _, statErr := os.Lstat(final); statErr == nil {
			continue
		}
		switch err := os.Mkdir(partial, 0o755); {
		case err == nil:
			return final, partial, nil
		case errors.Is(err, fs.ErrExist):
		default:
			return "", "", err
		}
	}
}

func writeBackup(ctx context.Context, s plan.BackupStep, dir string, created time.Time) error {
	byRoot := map[manifest.Root][]string{}
	meta := backup.Meta{Created: created, Direction: int(s.Direction), Roots: map[string]string{}}
	for _, f := range s.Files {
		byRoot[f.Root] = append(byRoot[f.Root], f.Path)
		meta.Roots[string(f.Root)] = s.Roots[f.Root]
		meta.Files = append(meta.Files, string(f.Root)+"/"+f.Path)
	}
	roots := make([]manifest.Root, 0, len(byRoot))
	for root := range byRoot {
		roots = append(roots, root)
	}
	slices.Sort(roots)
	for _, root := range roots {
		dst := filepath.Join(dir, string(root))
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		if err := rsync.Apply(ctx, rsync.Request{Src: s.Roots[root], Dst: dst, Dir: true}, byRoot[root], nil); err != nil {
			return err
		}
	}
	data, err := toml.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, backup.MetaFileName), data, 0o644)
}

func (r Runner) deleteBackup(s plan.DeleteBackupStep) plan.StepResult {
	dir := filepath.Clean(s.Dir)
	if filepath.Dir(dir) != backup.BackupsDir(r.Home) {
		return failed(fmt.Errorf("refusing to delete %s: not inside %s", dir, backup.BackupsDir(r.Home)))
	}
	if err := os.RemoveAll(dir); err != nil {
		return failed(err)
	}
	return plan.StepResult{Status: plan.StepOK}
}
