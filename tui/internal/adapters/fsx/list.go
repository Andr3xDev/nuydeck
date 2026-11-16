package fsx

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	backupsDirName = ".nuydeck-backups"
	metaFileName   = "meta.toml"
	partialSuffix  = ".partial"
)

type backupMeta struct {
	Created   time.Time         `toml:"created"`
	Direction int               `toml:"direction"`
	Roots     map[string]string `toml:"roots"`
	Files     []string          `toml:"files"`
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
			b.Created, b.Direction, b.Files = meta.Created, plan.Direction(meta.Direction), meta.Files
			b.Roots = map[manifest.Root]string{}
			for root, dest := range meta.Roots {
				b.Roots[manifest.Root(root)] = dest
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
