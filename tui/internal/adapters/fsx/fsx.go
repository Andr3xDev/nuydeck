package fsx

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/rsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const (
	backupsDirName   = ".nuydeck-backups"
	metaFileName     = "meta.toml"
	partialSuffix    = ".partial"
	backupTimeLayout = "20060102-150405"
	defaultVariant   = "nadir"
	keptNotEmpty     = "kept (not empty)"
)

// Runner executes DeleteStep, BackupStep and DeleteBackupStep. Backups live in
// <Home>/.nuydeck-backups. Now defaults to time.Now when nil.
type Runner struct {
	Home string
	Now  func() time.Time
}

type backupMeta struct {
	Created   time.Time         `toml:"created"`
	Direction int               `toml:"direction"`
	Roots     map[string]string `toml:"roots"`
	Files     []string          `toml:"files"`
}

func failed(err error) domain.StepResult {
	return domain.StepResult{Status: domain.StepFailed, Err: err}
}

func (r Runner) Run(ctx context.Context, s domain.Step, _ func(domain.StepProgress)) domain.StepResult {
	switch s := s.(type) {
	case domain.DeleteStep:
		return deletePath(s)
	case domain.BackupStep:
		return r.backup(ctx, s)
	case domain.DeleteBackupStep:
		return r.deleteBackup(s)
	case domain.WriteFileStep:
		return writeFile(s)
	case domain.SymlinkStep:
		return symlink(s)
	case domain.WriteSecretsStep:
		return writeSecrets(s)
	}
	return failed(fmt.Errorf("fsx cannot run %T", s))
}

func deletePath(s domain.DeleteStep) domain.StepResult {
	root, target := filepath.Clean(s.Root), filepath.Clean(s.Path)
	if !StrictlyWithin(root, target) {
		return failed(fmt.Errorf("refusing to delete %s: not inside %s", target, root))
	}
	// A symlinked parent would redirect the removal outside the root.
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return failed(err)
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if errors.Is(err, fs.ErrNotExist) {
		return domain.StepResult{Status: domain.StepOK, Detail: "already absent"}
	}
	if err != nil {
		return failed(err)
	}
	if !Within(realRoot, realParent) {
		return failed(fmt.Errorf("refusing to delete %s: parent resolves outside %s", target, root))
	}
	switch err := os.Remove(target); {
	case err == nil:
		return domain.StepResult{Status: domain.StepOK}
	case errors.Is(err, fs.ErrNotExist):
		return domain.StepResult{Status: domain.StepOK, Detail: "already absent"}
	case errors.Is(err, syscall.ENOTEMPTY):
		return domain.StepResult{Status: domain.StepOK, Detail: keptNotEmpty}
	default:
		return failed(err)
	}
}

func (r Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r Runner) backup(ctx context.Context, s domain.BackupStep) domain.StepResult {
	backupsDir := BackupsDir(r.Home)
	realBackupsDir, err := ResolveExisting(backupsDir)
	if err != nil {
		return failed(err)
	}
	for _, f := range s.Files {
		realEntry, err := ResolveExisting(filepath.Join(s.Roots[f.Root], f.Path))
		if err != nil {
			return failed(err)
		}
		if Within(realEntry, realBackupsDir) || Within(realBackupsDir, realEntry) {
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
	return domain.StepResult{Status: domain.StepOK, Detail: final}
}

// reserveBackupDir creates the .partial folder for the first free name among
// ts, ts-1, ts-2, ...
func reserveBackupDir(backupsDir, ts string) (final, partial string, err error) {
	for i := 0; ; i++ {
		name := ts
		if i > 0 {
			name = fmt.Sprintf("%s-%d", ts, i)
		}
		final, partial = filepath.Join(backupsDir, name), filepath.Join(backupsDir, name+partialSuffix)
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

func writeBackup(ctx context.Context, s domain.BackupStep, dir string, created time.Time) error {
	byRoot := map[domain.Root][]string{}
	meta := backupMeta{Created: created, Direction: int(s.Direction), Roots: map[string]string{}}
	for _, f := range s.Files {
		byRoot[f.Root] = append(byRoot[f.Root], f.Path)
		meta.Roots[string(f.Root)] = s.Roots[f.Root]
		meta.Files = append(meta.Files, string(f.Root)+"/"+f.Path)
	}
	roots := make([]domain.Root, 0, len(byRoot))
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
	return os.WriteFile(filepath.Join(dir, metaFileName), data, 0o644)
}

func (r Runner) deleteBackup(s domain.DeleteBackupStep) domain.StepResult {
	dir := filepath.Clean(s.Dir)
	if filepath.Dir(dir) != BackupsDir(r.Home) {
		return failed(fmt.Errorf("refusing to delete %s: not inside %s", dir, BackupsDir(r.Home)))
	}
	if err := os.RemoveAll(dir); err != nil {
		return failed(err)
	}
	return domain.StepResult{Status: domain.StepOK}
}

func BackupsDir(home string) string { return filepath.Join(home, backupsDirName) }

// ListBackups returns backups newest first; folders that cannot be read as a
// backup are flagged rather than dropped.
func ListBackups(_ context.Context, home string) ([]domain.Backup, error) {
	dir := BackupsDir(home)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var backups []domain.Backup
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b := domain.Backup{Dir: filepath.Join(dir, e.Name())}
		switch meta, err := readMeta(b.Dir); {
		case strings.HasSuffix(e.Name(), partialSuffix):
			b.Incomplete = true
		case err != nil:
			b.Unreadable = true
		default:
			b.Created, b.Direction, b.Files = meta.Created, domain.Direction(meta.Direction), meta.Files
			b.Roots = map[domain.Root]string{}
			for root, dest := range meta.Roots {
				b.Roots[domain.Root(root)] = dest
			}
		}
		backups = append(backups, b)
	}
	sort.SliceStable(backups, func(i, j int) bool {
		if !backups[i].Created.Equal(backups[j].Created) {
			return backups[i].Created.After(backups[j].Created)
		}
		return backups[i].Dir > backups[j].Dir
	})
	return backups, nil
}

func readMeta(dir string) (backupMeta, error) {
	var meta backupMeta
	data, err := os.ReadFile(filepath.Join(dir, metaFileName))
	if err != nil {
		return meta, err
	}
	return meta, toml.Unmarshal(data, &meta)
}

// PathKind classifies p by Lstat; a missing path is KindNone, not an error.
func PathKind(p string) (domain.NodeKind, error) {
	info, err := os.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return domain.KindNone, nil
	case err != nil:
		return domain.KindNone, err
	case info.Mode()&fs.ModeSymlink != 0:
		return domain.KindLink, nil
	case info.IsDir():
		return domain.KindDir, nil
	}
	return domain.KindFile, nil
}

// ReadVariant returns the theme variant, "nadir" unless the file says "zenit".
func ReadVariant(configDir string) string {
	data, err := os.ReadFile(filepath.Join(configDir, "nuydeck", "theme", "variant"))
	if err == nil && strings.TrimSpace(string(data)) == "zenit" {
		return "zenit"
	}
	return defaultVariant
}

// LinkTarget returns the raw target of the symlink at p, without following it.
func LinkTarget(p string) (string, error) { return os.Readlink(p) }

// NewestMTime returns the newest mtime of the files and links under root,
// ignoring paths the filter excludes. Symlinked dirs are not followed; a
// missing root yields the zero time.
func NewestMTime(root string, filter domain.Filter) (time.Time, error) {
	var newest time.Time
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel != "." && domain.Excluded(filepath.ToSlash(rel), d.IsDir(), filter) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) && newest.IsZero() {
		return time.Time{}, nil
	}
	return newest, err
}
