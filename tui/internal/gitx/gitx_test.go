package gitx_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/gitx"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

func git(t *testing.T, repo string, date time.Time, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	stamp := date.Format(time.RFC3339)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, repo, rel string, date time.Time) {
	t.Helper()
	write(t, filepath.Join(repo, rel), date.String())
	git(t, repo, date, "add", "--", rel)
	git(t, repo, date, "commit", "-m", "c "+rel)
}

var (
	t1 = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 = time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
)

func newRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	repo := t.TempDir()
	git(t, repo, t1, "init", "-q")
	return repo
}

func TestRepoTimes_CancelledContextReturnsCtxErrNotUnknown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := gitx.Reader{Git: func(ctx context.Context, _ ...string) ([]byte, error) { return nil, ctx.Err() }}

	got, err := r.RepoTimes(ctx, t.TempDir(), "x")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got.Unknown {
		t.Fatal("cancelled run reported as Unknown")
	}
}

func TestRepoTimes_CleanEntryUsesCommitTimes(t *testing.T) {
	repo := newRepo(t)
	commit(t, repo, "cfg/a.txt", t1)
	commit(t, repo, "cfg/b.txt", t2)

	got, err := gitx.Reader{}.RepoTimes(context.Background(), repo, "cfg")

	if err != nil || got.Unknown {
		t.Fatalf("err = %v, unknown = %v", err, got.Unknown)
	}
	if !got.Entry.Equal(t2) || !got.Files["a.txt"].Equal(t1) || !got.Files["b.txt"].Equal(t2) {
		t.Fatalf("got entry %v files %v", got.Entry, got.Files)
	}
}

func TestRepoTimes_DirtyAndUntrackedFilesUseMtimeAndOtherPathsAreIgnored(t *testing.T) {
	repo := newRepo(t)
	commit(t, repo, "cfg/a.txt", t1)
	commit(t, repo, "cfg/b.txt", t2)
	commit(t, repo, "other/x.txt", t2)
	touch := func(rel string, ts time.Time, content string) {
		p := filepath.Join(repo, rel)
		write(t, p, content)
		if err := os.Chtimes(p, ts, ts); err != nil {
			t.Fatal(err)
		}
	}
	touch("cfg/a.txt", t3, "edited")
	touch("cfg/new.txt", t3.Add(time.Hour), "untracked")
	touch("other/x.txt", t3.Add(24*time.Hour), "edited elsewhere")

	got, err := gitx.Reader{}.RepoTimes(context.Background(), repo, "cfg")

	if err != nil {
		t.Fatal(err)
	}
	if !got.Files["a.txt"].Equal(t3) || !got.Files["new.txt"].Equal(t3.Add(time.Hour)) || !got.Files["b.txt"].Equal(t2) {
		t.Fatalf("files = %v", got.Files)
	}
	if !got.Entry.Equal(t3.Add(time.Hour)) {
		t.Fatalf("entry = %v, want newest dirty mtime", got.Entry)
	}
}

func TestRepoTimes_QuotedNameFallsBackToEntryTime(t *testing.T) {
	repo := newRepo(t)
	commit(t, repo, `cfg/we"ird.txt`, t1)
	commit(t, repo, "cfg/b.txt", t2)

	got, err := gitx.Reader{}.RepoTimes(context.Background(), repo, "cfg")

	if err != nil || got.Unknown {
		t.Fatalf("err = %v, unknown = %v", err, got.Unknown)
	}
	if !got.Entry.Equal(t2) {
		t.Fatalf("entry = %v", got.Entry)
	}
	if ft, ok := got.Files[`we"ird.txt`]; ok && !ft.Equal(got.Entry) {
		t.Fatalf("quoted name resolved to %v, want absent or entry time", ft)
	}
}

func TestRepoTimes_NonGitDirIsUnknownWithoutError(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "cfg", "a.txt"), "x")

	got, err := gitx.Reader{}.RepoTimes(context.Background(), dir, "cfg")

	if err != nil || !got.Unknown {
		t.Fatalf("err = %v, unknown = %v; want unknown without error", err, got.Unknown)
	}
}

func TestRepoTimes_InvokesOnlyReadOnlyGitCommands(t *testing.T) {
	repo := newRepo(t)
	commit(t, repo, "cfg/a.txt", t1)
	var calls [][]string
	r := gitx.Reader{Git: func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return exec.CommandContext(ctx, "git", args...).Output()
	}}

	if _, err := r.RepoTimes(context.Background(), repo, "cfg"); err != nil {
		t.Fatal(err)
	}

	subcommands := map[string][]string{}
	for _, args := range calls {
		for i := 0; i < len(args); i++ {
			if args[i] == "-C" || args[i] == "-c" {
				i++
				continue
			}
			subcommands[args[i]] = args
			break
		}
	}
	if len(calls) == 0 {
		t.Fatal("git was never invoked")
	}
	for sub := range subcommands {
		if sub != "status" && sub != "log" {
			t.Fatalf("non-read-only git subcommand %q invoked", sub)
		}
	}
	if st := subcommands["status"]; !slices.Contains(st, "--porcelain=v1") || !slices.Contains(st, "-z") {
		t.Fatalf("status argv = %v", st)
	}
}
