package cfgsync

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/fsutil"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// The write step refuses a path equal to its root, so a file entry needs the parent directory as root.
func TestSteps_EditedFileEntryWriteRootIsStrictlyAboveTheFile(t *testing.T) {
	e := resolved(manifest.RootHome, ".zshrc")
	in := &fakeInspector{
		diff:  diffReturning(change(".zshrc", plan.Update, plan.KindFile)),
		kinds: map[string]plan.NodeKind{e.RepoPath: plan.KindFile},
	}
	p, err := newService(t, in, &fakeRunner{}).PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	it := p.Items[0]

	writes := stepsOfType[plan.WriteFileStep](flatten(Steps(p, selectAll(p), editOf(it), ApplyOptions{})))

	if len(writes) != 1 || writes[0].Path != e.DestPath || !fsutil.StrictlyWithin(writes[0].Root, writes[0].Path) {
		t.Fatalf("writes = %+v, want one write to %s with a root strictly above it", writes, e.DestPath)
	}
}

// A file entry is itemized by rsync under its base name; the entry endpoints are the file paths themselves.
func TestFileEntry_PlanAndApplyUseTheFilePathsDirectly(t *testing.T) {
	e := resolved(manifest.RootHome, ".zshrc")
	repoT := testNow.Add(-time.Hour)
	in := &fakeInspector{
		diff:      diffReturning(change(".zshrc", plan.Update, plan.KindFile)),
		kinds:     map[string]plan.NodeKind{e.RepoPath: plan.KindFile},
		repoTimes: map[string]plan.RepoTimes{"home/.zshrc": {Files: map[string]time.Time{".": repoT}}},
		mtimes:    map[string]time.Time{e.DestPath: testNow},
	}
	r := &fakeRunner{}
	svc := newService(t, in, r)
	syncPlan, err := svc.PlanSync(context.Background(), plan.Push, []manifest.ResolvedEntry{e})
	if err != nil {
		t.Fatal(err)
	}
	it := syncPlan.Items[0]
	if it.Key() != (ItemKey{Root: manifest.RootHome, Path: ".zshrc"}) || it.Blocked || !it.RepoTime.Equal(repoT) || !it.PCTime.Equal(testNow) {
		t.Fatalf("item = %+v key = %+v", it, it.Key())
	}

	svc.Apply(context.Background(), syncPlan, selectAll(syncPlan), ApplyOptions{Backup: true}, nil)

	copies := stepsOfType[plan.CopyStep](r.steps)
	if len(copies) != 1 || copies[0].Src != e.RepoPath || copies[0].Dst != e.DestPath || !reflect.DeepEqual(copies[0].Paths, []string{".zshrc"}) || copies[0].Dir {
		t.Fatalf("copy steps = %+v", copies)
	}
	backups := stepsOfType[plan.BackupStep](r.steps)
	want := plan.BackupFile{Root: manifest.RootHome, Path: ".zshrc"}
	if len(backups) != 1 || !reflect.DeepEqual(backups[0].Files, []plan.BackupFile{want}) || backups[0].Roots[manifest.RootHome] != testHome {
		t.Fatalf("backup steps = %+v", backups)
	}
}
