package app

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

// A file entry is itemized by rsync under its base name; the entry endpoints are the file paths themselves.
func TestFileEntry_PlanAndApplyUseTheFilePathsDirectly(t *testing.T) {
	e := resolved(manifest.RootHome, ".zshrc")
	repoT := testNow.Add(-time.Hour)
	in := &fakeInspector{
		diff:      diffReturning(change(".zshrc", domain.Update, domain.KindFile)),
		kinds:     map[string]domain.NodeKind{e.RepoPath: domain.KindFile},
		repoTimes: map[string]domain.RepoTimes{"home/.zshrc": {Files: map[string]time.Time{".": repoT}}},
		mtimes:    map[string]time.Time{e.DestPath: testNow},
	}
	r := &fakeRunner{}
	svc := newService(t, in, r)
	plan, err := svc.PlanSync(context.Background(), domain.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	it := plan.Items[0]
	if it.Key() != (ItemKey{Root: manifest.RootHome, Path: ".zshrc"}) || it.Blocked || !it.RepoTime.Equal(repoT) || !it.PCTime.Equal(testNow) {
		t.Fatalf("item = %+v key = %+v", it, it.Key())
	}

	svc.Apply(context.Background(), plan, selectAll(plan), ApplyOptions{Backup: true}, nil)

	copies := stepsOfType[domain.CopyStep](r.steps)
	if len(copies) != 1 || copies[0].Src != e.RepoPath || copies[0].Dst != e.DestPath || !reflect.DeepEqual(copies[0].Paths, []string{".zshrc"}) || copies[0].Dir {
		t.Fatalf("copy steps = %+v", copies)
	}
	backups := stepsOfType[domain.BackupStep](r.steps)
	want := domain.BackupFile{Root: manifest.RootHome, Path: ".zshrc"}
	if len(backups) != 1 || !reflect.DeepEqual(backups[0].Files, []domain.BackupFile{want}) || backups[0].Roots[manifest.RootHome] != testHome {
		t.Fatalf("backup steps = %+v", backups)
	}
}
