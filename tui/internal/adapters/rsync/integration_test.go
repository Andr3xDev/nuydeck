package rsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
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

func planItems(t *testing.T, r Request) []Item {
	t.Helper()
	res, err := Plan(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return res.Items
}

func apply(t *testing.T, r Request, paths ...string) {
	t.Helper()
	if err := Apply(context.Background(), r, paths, nil); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationApplyReportsProgressPerItem(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	write(t, root+"/repo/a", "1", 0o644)
	write(t, root+"/repo/b", "2", 0o644)
	var got []string
	err := Apply(context.Background(), dirReq(root+"/repo", root+"/home"), []string{"a", "b"}, func(p string) { got = append(got, p) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("progress = %v, want a and b", got)
	}
}

func dirReq(src, dst string, excludes ...string) Request {
	return Request{Src: src, Dst: dst, Dir: true, Filter: manifest.Filter{Excludes: excludes}}
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
	got := planItems(t, r)
	want := []Item{
		{"extra.txt", plan.Delete, plan.KindFile},
		{"new.txt", plan.Create, plan.KindFile},
		{"rel", plan.Create, plan.KindLink},
		{"sub/deep.txt", plan.Create, plan.KindFile},
		{"upd.txt", plan.Update, plan.KindFile},
	}
	slices.SortFunc(got, func(a, b Item) int { return strings.Compare(a.Path, b.Path) })
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

func TestIntegrationApplyFileEntry(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	write(t, root+"/repo/.zshrc", "new", 0o644)
	write(t, root+"/home/.zshrc", "old", 0o644)
	apply(t, Request{Src: root + "/repo/.zshrc", Dst: root + "/home/.zshrc"})
	if read(t, root+"/home/.zshrc") != "new" {
		t.Fatal("file entry not synced")
	}
}

func TestIntegrationPlanErrorIsSingleLine(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()

	_, err := Plan(context.Background(), dirReq(root+"/missing-src", root+"/dst"))
	if err == nil {
		t.Fatal("expected an error for a missing source")
	}
	if strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("plan error must be one line: %q", err)
	}
}

func TestIntegrationApplyFailureIsError(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	r := dirReq(root+"/missing", root+"/dst")
	if err := Apply(context.Background(), r, []string{"a"}, nil); err == nil {
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

	got := planItems(t, dirReq(src, dst, "/identities.json"))
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

	if got := planItems(t, r); len(got) != 1 || got[0].Type != plan.Update {
		t.Fatalf("exec bit difference must be one update, got %+v", got)
	}
	apply(t, r, "run.sh")
	if info, _ := os.Stat(dst + "/run.sh"); info.Mode().Perm()&0o111 == 0 {
		t.Fatal("exec bit not synced")
	}
	if got := planItems(t, r); len(got) != 0 {
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

	got := planItems(t, dirReq(src, dst))
	want := []Item{{"content.txt", plan.Update, plan.KindFile}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestIntegrationPlanSkipsFifoWithWarning(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	src, dst := root+"/src", root+"/dst"
	write(t, src+"/a.txt", "a", 0o644)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(src+"/pipe", 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Plan(context.Background(), dirReq(src, dst))
	if err != nil {
		t.Fatal(err)
	}
	if want := []Item{{"a.txt", plan.Create, plan.KindFile}}; !slices.Equal(res.Items, want) {
		t.Fatalf("got %+v want %+v", res.Items, want)
	}
	if !strings.Contains(res.Warning, "pipe") {
		t.Fatalf("warning must name the skipped fifo, got %q", res.Warning)
	}
}

func TestIntegrationPlanUnreadableSourceIsError(t *testing.T) {
	requireRsync(t)
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	root := t.TempDir()
	src, dst := root+"/src", root+"/dst"
	write(t, src+"/locked/secret.txt", "s", 0o644)
	// A dry-run lists unreadable files but still fails (exit 23) on an unreadable directory.
	if err := os.Chmod(src+"/locked", 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(src+"/locked", 0o755) })

	if _, err := Plan(context.Background(), dirReq(src, dst)); err == nil {
		t.Fatal("want error when a source directory is unreadable")
	}
}

func TestIntegrationApplyDirWithNoPathsIsNoOp(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	r := dirReq(root+"/missing", root+"/dst")
	if err := Apply(context.Background(), r, nil, nil); err != nil {
		t.Fatalf("empty selection must not invoke rsync, got %v", err)
	}
}
