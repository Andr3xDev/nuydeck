package rsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func requireRsync(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not installed")
	}
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func setMtime(t *testing.T, path string, ts time.Time) {
	t.Helper()
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatal(err)
	}
}

func plan(t *testing.T, r Request) []Item {
	t.Helper()
	res, err := Plan(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return res.Items
}

func apply(t *testing.T, r Request, paths ...string) {
	t.Helper()
	if err := Apply(context.Background(), r, paths); err != nil {
		t.Fatal(err)
	}
}

func dirReq(src, dst string, excludes ...string) Request {
	return Request{Src: src, Dst: dst, Dir: true, Filter: domain.Filter{Excludes: excludes}}
}

func TestIntegrationPlanApplyTouchesOnlySelected(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	write(t, src+"/new.txt", "new", 0o644)
	write(t, src+"/upd.txt", "after", 0o644)
	write(t, src+"/sub/deep.txt", "deep", 0o644)
	symlink(t, "sub/deep.txt", src+"/rel")
	write(t, dst+"/upd.txt", "before", 0o644)
	write(t, dst+"/extra.txt", "extra", 0o644)

	r := dirReq(src, dst)
	got := plan(t, r)
	want := []Item{
		{"extra.txt", domain.Delete, domain.KindFile},
		{"new.txt", domain.Create, domain.KindFile},
		{"rel", domain.Create, domain.KindLink},
		{"sub/deep.txt", domain.Create, domain.KindFile},
		{"upd.txt", domain.Update, domain.KindFile},
	}
	slices.SortFunc(got, func(a, b Item) int { return compareStrings(a.Path, b.Path) })
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}

	apply(t, r, "new.txt", "rel", "sub/deep.txt")
	if read(t, dst+"/new.txt") != "new" || read(t, dst+"/sub/deep.txt") != "deep" {
		t.Fatal("selected files not copied")
	}
	if target, err := os.Readlink(dst + "/rel"); err != nil || target != "sub/deep.txt" {
		t.Fatalf("relative link must be copied as a link, got %q, %v", target, err)
	}
	if read(t, dst+"/upd.txt") != "before" {
		t.Fatal("unselected update was applied")
	}
	if _, err := os.Stat(dst + "/extra.txt"); err != nil {
		t.Fatal("Apply must never delete")
	}
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func TestIntegrationApplyFileEntry(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	write(t, root+"/repo/.zshrc", "new", 0o644)
	write(t, root+"/home/.zshrc", "old", 0o644)
	apply(t, Request{Src: root + "/repo/.zshrc", Dst: root + "/home/.zshrc"}, nil...)
	if read(t, root+"/home/.zshrc") != "new" {
		t.Fatal("file entry not synced")
	}
}

func TestIntegrationApplyFailureIsError(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	r := dirReq(root+"/missing", root+"/dst")
	if err := Apply(context.Background(), r, []string{"a"}); err == nil {
		t.Fatal("want error for missing source")
	}
}

func TestIntegrationPullThroughRepoSymlinkLeavesLiveFileAlone(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	pc, repo, live := root+"/pc", root+"/repo", root+"/live/starship.toml"
	write(t, live, "live content", 0o644)
	symlink(t, "variants/nadir.toml", pc+"/starship.toml")
	symlink(t, live, repo+"/starship.toml")

	apply(t, dirReq(pc, repo), "starship.toml")

	if read(t, live) != "live content" {
		t.Fatal("live file was modified through the repo symlink")
	}
	if target, err := os.Readlink(repo + "/starship.toml"); err != nil || target != "variants/nadir.toml" {
		t.Fatalf("repo path must become a copy of the link, got %q, %v", target, err)
	}
}

func TestIntegrationPushThroughPCSymlinkLeavesTargetAlone(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	repo, pc, outside := root+"/repo", root+"/pc", root+"/outside.toml"
	write(t, outside, "outside content", 0o644)
	write(t, repo+"/app.toml", "from repo", 0o644)
	symlink(t, outside, pc+"/app.toml")

	apply(t, dirReq(repo, pc), "app.toml")

	if read(t, outside) != "outside content" {
		t.Fatal("symlink target was modified")
	}
	if info, err := os.Lstat(pc + "/app.toml"); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("PC path must become a regular file, got %v, %v", info, err)
	}
	if read(t, pc+"/app.toml") != "from repo" {
		t.Fatal("PC file not replaced")
	}
}

func TestIntegrationExcludedReceiverOnlyFileIsNotOffered(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	src, dst := root+"/src", root+"/dst"
	write(t, src+"/keep.txt", "k", 0o644)
	write(t, dst+"/identities.json", "secret", 0o600)

	got := plan(t, dirReq(src, dst, "/identities.json"))
	for _, it := range got {
		if it.Path == "identities.json" {
			t.Fatalf("excluded receiver-only file offered: %+v", it)
		}
	}
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestIntegrationExecutabilityIsSyncedWithoutMtimeDrift(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	src, dst := root+"/src", root+"/dst"
	write(t, src+"/run.sh", "same", 0o755)
	write(t, dst+"/run.sh", "same", 0o644)
	setMtime(t, src+"/run.sh", time.Unix(1_000_000_000, 0))
	setMtime(t, dst+"/run.sh", time.Unix(1_500_000_000, 0))
	r := dirReq(src, dst)

	if got := plan(t, r); len(got) != 1 || got[0].Type != domain.Update {
		t.Fatalf("exec bit difference must be one update, got %+v", got)
	}
	apply(t, r, "run.sh")
	if info, _ := os.Stat(dst + "/run.sh"); info.Mode().Perm()&0o111 == 0 {
		t.Fatal("exec bit not synced")
	}
	if got := plan(t, r); len(got) != 0 {
		t.Fatalf("mtime difference must not cause drift, got %+v", got)
	}
}

func TestIntegrationChecksumDecidesSync(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	src, dst := root+"/src", root+"/dst"
	same := time.Unix(1_000_000_000, 0)
	write(t, src+"/content.txt", "aaaa", 0o644)
	write(t, dst+"/content.txt", "bbbb", 0o644)
	write(t, src+"/mtime.txt", "same", 0o644)
	write(t, dst+"/mtime.txt", "same", 0o644)
	setMtime(t, src+"/content.txt", same)
	setMtime(t, dst+"/content.txt", same)
	setMtime(t, src+"/mtime.txt", same)
	setMtime(t, dst+"/mtime.txt", same.Add(time.Hour))

	got := plan(t, dirReq(src, dst))
	want := []Item{{"content.txt", domain.Update, domain.KindFile}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
