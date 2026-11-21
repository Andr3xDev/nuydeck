package selfupdate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	oldBinary = "old binary\n"
	newBinary = "new binary\n"
)

// goStub stands in for the go toolchain: it logs every call and, for "go build -o OUT", writes OUT unless GO_STUB_FAIL=1.
const goStub = `#!/bin/bash
echo "$@" >> "$GO_STUB_LOG"
[ "${1:-}" = build ] || exit 0
if [ "${GO_STUB_FAIL:-}" = 1 ]; then echo "stub build failed" >&2; exit 1; fi
while [ $# -gt 0 ]; do
  if [ "$1" = -o ]; then out=$2; fi
  shift
done
echo "new binary" > "$out"
`

func requireTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"git", "bash"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
}

// scriptEnv is a git sandbox with a stub go first on PATH.
type scriptEnv struct {
	t       *testing.T
	env     []string
	goLog   string
	root    string // working clone the script updates
	origin  string
	scratch string
}

func newScriptEnv(t *testing.T) *scriptEnv {
	t.Helper()
	requireTools(t)
	scratch := t.TempDir()
	stubDir := filepath.Join(scratch, "stub")
	writeTestFile(t, filepath.Join(stubDir, "go"), goStub, 0o755)
	e := &scriptEnv{
		t:       t,
		goLog:   filepath.Join(scratch, "go.log"),
		root:    filepath.Join(scratch, "work"),
		origin:  filepath.Join(scratch, "origin.git"),
		scratch: scratch,
	}
	e.env = append(os.Environ(),
		"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GO_STUB_LOG="+e.goLog,
		"HOME="+scratch,
		"GIT_CONFIG_GLOBAL="+filepath.Join(scratch, "gitconfig"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)

	e.git(scratch, "init", "--bare", "-b", "main", e.origin)
	e.git(scratch, "clone", e.origin, e.root)
	script, err := filepath.Abs("../../../update-tui.sh")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("update-tui.sh missing at the repo root: %v", err)
	}
	writeTestFile(t, filepath.Join(e.root, "update-tui.sh"), string(body), 0o755)
	writeTestFile(t, filepath.Join(e.root, "tracked.txt"), "v1\n", 0o644)
	writeTestFile(t, filepath.Join(e.root, ".gitignore"), "tui/bin/\n", 0o644)
	e.git(e.root, "add", ".")
	e.git(e.root, "commit", "-m", "initial")
	e.git(e.root, "push", "-u", "origin", "main")
	writeTestFile(t, e.binary(), oldBinary, 0o755)
	return e
}

func (e *scriptEnv) binary() string { return filepath.Join(e.root, "tui", "bin", "nuydeck") }

func (e *scriptEnv) git(dir string, args ...string) string {
	e.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, e.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// pushUpstreamCommit makes origin one commit ahead of the working clone.
func (e *scriptEnv) pushUpstreamCommit() {
	e.t.Helper()
	other := filepath.Join(e.scratch, "other")
	e.git(e.scratch, "clone", e.origin, other)
	writeTestFile(e.t, filepath.Join(other, "upstream.txt"), "upstream\n", 0o644)
	e.git(other, "add", ".")
	e.git(other, "commit", "-m", "upstream")
	e.git(other, "push", "origin", "main")
}

// run executes the script and returns its combined output and exit code.
func (e *scriptEnv) run(extraEnv ...string) (string, int) {
	e.t.Helper()
	cmd := exec.Command("bash", filepath.Join(e.root, "update-tui.sh"))
	cmd.Dir, cmd.Env = e.root, append(slicesClone(e.env), extraEnv...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exit):
		return string(out), exit.ExitCode()
	}
	e.t.Fatalf("running update-tui.sh: %v", err)
	return "", -1
}

func (e *scriptEnv) buildCalls() int {
	log, err := os.ReadFile(e.goLog)
	if err != nil {
		return 0
	}
	calls := 0
	for _, line := range strings.Split(string(log), "\n") {
		if strings.HasPrefix(line, "build") {
			calls++
		}
	}
	return calls
}

func (e *scriptEnv) requireBinary(want string) {
	e.t.Helper()
	got, err := os.ReadFile(e.binary())
	if err != nil || string(got) != want {
		e.t.Errorf("bin/nuydeck = %q, %v, want %q", got, err, want)
	}
	if _, err := os.Stat(e.binary() + ".new"); err == nil {
		e.t.Error("bin/nuydeck.new left behind")
	}
}

func slicesClone(s []string) []string { return append([]string(nil), s...) }

func writeTestFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateScript_DirtyTrackedFileIsRefusedBeforePullAndBuild(t *testing.T) {
	e := newScriptEnv(t)
	e.pushUpstreamCommit()
	head := e.git(e.root, "rev-parse", "HEAD")
	writeTestFile(t, filepath.Join(e.root, "tracked.txt"), "edited\n", 0o644)

	out, code := e.run()

	if code == 0 {
		t.Fatalf("exit = 0, want a refusal\n%s", out)
	}
	if got := e.git(e.root, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved to %s, want the pull skipped", got)
	}
	if e.buildCalls() != 0 {
		t.Error("go build ran on a dirty tree")
	}
	e.requireBinary(oldBinary)
}

func TestUpdateScript_UntrackedFileDoesNotBlockTheUpdate(t *testing.T) {
	e := newScriptEnv(t)
	writeTestFile(t, filepath.Join(e.root, "notes.txt"), "scratch\n", 0o644)

	out, code := e.run()

	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, out)
	}
	e.requireBinary(newBinary)
}

func TestUpdateScript_NonFastForwardPullStopsBeforeBuildAndKeepsBinary(t *testing.T) {
	e := newScriptEnv(t)
	e.pushUpstreamCommit()
	writeTestFile(t, filepath.Join(e.root, "local.txt"), "local\n", 0o644)
	e.git(e.root, "add", "local.txt")
	e.git(e.root, "commit", "-m", "local")

	out, code := e.run()

	if code == 0 {
		t.Fatalf("exit = 0, want the pull to fail\n%s", out)
	}
	if e.buildCalls() != 0 {
		t.Error("go build ran after a failed pull")
	}
	e.requireBinary(oldBinary)
}

func TestUpdateScript_FailedBuildKeepsTheCurrentBinary(t *testing.T) {
	e := newScriptEnv(t)
	e.pushUpstreamCommit()

	out, code := e.run("GO_STUB_FAIL=1")

	if code == 0 {
		t.Fatalf("exit = 0, want the build failure to propagate\n%s", out)
	}
	if e.buildCalls() != 1 {
		t.Errorf("go build calls = %d, want 1", e.buildCalls())
	}
	e.requireBinary(oldBinary)
}

func TestUpdateScript_BranchWithoutUpstreamPullsFromOriginBranch(t *testing.T) {
	e := newScriptEnv(t)
	e.git(e.root, "checkout", "-b", "feature")
	e.git(e.root, "push", "origin", "feature")
	other := filepath.Join(e.scratch, "other")
	e.git(e.scratch, "clone", "-b", "feature", e.origin, other)
	writeTestFile(t, filepath.Join(other, "feature.txt"), "feature\n", 0o644)
	e.git(other, "add", ".")
	e.git(other, "commit", "-m", "feature work")
	e.git(other, "push", "origin", "feature")

	out, code := e.run()

	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(e.root, "feature.txt")); err != nil {
		t.Errorf("remote branch commit not pulled: %v", err)
	}
	e.requireBinary(newBinary)
}

func TestUpdateScript_LocalOnlyBranchWarnsSkipsPullAndBuilds(t *testing.T) {
	e := newScriptEnv(t)
	e.pushUpstreamCommit()
	e.git(e.root, "checkout", "-b", "local-only")

	out, code := e.run()

	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "no remote branch for local-only") {
		t.Errorf("missing warning in output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(e.root, "upstream.txt")); err == nil {
		t.Error("pull ran for a branch with no remote counterpart")
	}
	e.requireBinary(newBinary)
}

func TestUpdateScript_UnreachableOriginOnBranchWithoutUpstreamAbortsBeforeBuild(t *testing.T) {
	e := newScriptEnv(t)
	e.git(e.root, "checkout", "-b", "local-only")
	e.git(e.root, "remote", "set-url", "origin", filepath.Join(e.scratch, "missing.git"))

	out, code := e.run()

	if code == 0 {
		t.Fatalf("exit = 0, want the unreachable origin to abort\n%s", out)
	}
	if e.buildCalls() != 0 {
		t.Error("go build ran after a failed remote check")
	}
	e.requireBinary(oldBinary)
}

func TestUpdateScript_SuccessPullsAndReplacesTheBinary(t *testing.T) {
	e := newScriptEnv(t)
	e.pushUpstreamCommit()

	out, code := e.run()

	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(e.root, "upstream.txt")); err != nil {
		t.Errorf("upstream commit not pulled: %v", err)
	}
	e.requireBinary(newBinary)
}
