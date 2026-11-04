package rsync

import (
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

var forbiddenFlags = []string{"-a", "--archive", "--inplace", "-K", "--keep-dirlinks", "--safe-links", "--delete-excluded"}

func dirRequest() Request {
	return Request{Src: "/repo/cfg", Dst: "/home/u/.config/cfg", Dir: true,
		Filter: domain.Filter{Excludes: []string{"/identities.json", "/cache"}}}
}

func assertNone(t *testing.T, args []string, flags []string) {
	t.Helper()
	for _, f := range flags {
		if slices.Contains(args, f) {
			t.Errorf("forbidden flag %s in %v", f, args)
		}
	}
}

func assertAll(t *testing.T, args []string, flags []string) {
	t.Helper()
	for _, f := range flags {
		if !slices.Contains(args, f) {
			t.Errorf("missing flag %s in %v", f, args)
		}
	}
}

func TestPlanArgs_DirEntry(t *testing.T) {
	args := PlanArgs(dirRequest())
	assertAll(t, args, []string{"--recursive", "--links", "--checksum", "--executability",
		"--dry-run", "--itemize-changes", "--delete", "--8-bit-output",
		"--exclude=/identities.json", "--exclude=/cache"})
	assertNone(t, args, forbiddenFlags)
	if got := args[len(args)-3:]; got[0] != "--" || got[1] != "/repo/cfg/" || got[2] != "/home/u/.config/cfg/" {
		t.Fatalf("dir entry must end with -- SRC/ DST/, got %v", got)
	}
}

func TestPlanArgs_FileEntryKeepsPathsWithoutSlash(t *testing.T) {
	args := PlanArgs(Request{Src: "/repo/.zshrc", Dst: "/home/u/.zshrc"})
	if got := args[len(args)-3:]; got[0] != "--" || got[1] != "/repo/.zshrc" || got[2] != "/home/u/.zshrc" {
		t.Fatalf("got %v", got)
	}
}

func TestApplyArgs_NeverDeletes(t *testing.T) {
	args := ApplyArgs(dirRequest(), "/tmp/list")
	assertAll(t, args, []string{"--recursive", "--links", "--checksum", "--executability",
		"--itemize-changes", "--8-bit-output", "--files-from=/tmp/list", "--from0",
		"--exclude=/identities.json", "--exclude=/cache"})
	assertNone(t, args, append([]string{"--delete", "--dry-run"}, forbiddenFlags...))
	if got := args[len(args)-3:]; got[0] != "--" || got[1] != "/repo/cfg/" || got[2] != "/home/u/.config/cfg/" {
		t.Fatalf("must end with -- SRC/ DST/, got %v", got)
	}
}

func TestApplyArgs_FileEntryHasNoFilesFrom(t *testing.T) {
	args := ApplyArgs(Request{Src: "/repo/.zshrc", Dst: "/home/u/.zshrc"}, "/tmp/list")
	for _, a := range args {
		if a == "--from0" || a == "--files-from=/tmp/list" {
			t.Fatalf("unexpected %s in %v", a, args)
		}
	}
}
