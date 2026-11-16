package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// commitMarker prefixes the commit-time line of git log output; names printed
// by git never start with a control character because git quotes them.
const commitMarker = "\x01"

// Reader runs read-only git commands. Git receives the argv after the "git"
// binary (starting with -C <repo>); nil means exec the real git.
type Reader struct {
	Git func(ctx context.Context, args ...string) ([]byte, error)
}

func runGit(ctx context.Context, args ...string) ([]byte, error) {
	var out bytes.Buffer
	res, err := execx.Run(ctx, &out, "git", args...)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git exited with code %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return out.Bytes(), nil
}

// RepoTimes returns last-updated times for the files under rel: mtime for
// dirty or untracked files, last commit time otherwise. Files keys are
// relative to rel. A git failure (e.g. not a repository) yields Unknown.
func (r Reader) RepoTimes(ctx context.Context, repo, rel string) (plan.RepoTimes, error) {
	git := r.Git
	if git == nil {
		git = runGit
	}
	gitFailed := func() (plan.RepoTimes, error) {
		if ctx.Err() != nil {
			return plan.RepoTimes{}, ctx.Err()
		}
		return plan.RepoTimes{Unknown: true}, nil
	}
	statusOut, err := git(ctx, "-C", repo, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", rel)
	if err != nil {
		return gitFailed()
	}
	logOut, err := git(ctx, "-C", repo, "log", "--format="+commitMarker+"%ct", "--name-only", "--no-renames", "--", rel)
	if err != nil {
		return gitFailed()
	}

	times := plan.RepoTimes{Files: map[string]time.Time{}}
	var newestCommit, newestDirty time.Time
	var commitTime time.Time
	for _, line := range strings.Split(string(logOut), "\n") {
		switch ct, isCommit := strings.CutPrefix(line, commitMarker); {
		case isCommit:
			secs, err := strconv.ParseInt(ct, 10, 64)
			if err != nil {
				return plan.RepoTimes{Unknown: true}, nil
			}
			commitTime = time.Unix(secs, 0)
			if newestCommit.IsZero() {
				newestCommit = commitTime
			}
		case line == "" || strings.HasPrefix(line, `"`):
			// Quoted names are not decoded; the file falls back to the entry time.
		default:
			key := relativeTo(rel, line)
			if _, seen := times.Files[key]; !seen {
				times.Files[key] = commitTime
			}
		}
	}

	fields := strings.Split(string(statusOut), "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		if f[0] == 'R' || f[0] == 'C' {
			i++ // the rename/copy source follows as its own field
		}
		info, err := os.Lstat(filepath.Join(repo, f[3:]))
		if err != nil {
			continue
		}
		times.Files[relativeTo(rel, f[3:])] = info.ModTime()
		if info.ModTime().After(newestDirty) {
			newestDirty = info.ModTime()
		}
	}

	times.Entry = newestCommit
	if !newestDirty.IsZero() {
		times.Entry = newestDirty
	}
	return times, nil
}

func relativeTo(rel, name string) string {
	if r, err := filepath.Rel(rel, name); err == nil {
		return r
	}
	return name
}
