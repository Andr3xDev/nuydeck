package system_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/system"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

type execCall struct {
	name string
	args []string
	dir  string
	// installed holds the content of the temp file passed to `install`, read while it still exists.
	installed string
}

type fakeExec struct {
	calls []execCall
	// failOn maps a command word (cp, install, mv, ...) to a non-zero exit.
	failOn map[string]bool
	stdout string
}

func (f *fakeExec) run(_ context.Context, opts execx.Options, name string, args ...string) (execx.Result, error) {
	c := execCall{name: name, args: args, dir: opts.Dir}
	if sub := f.subcommand(c); sub == "install" && len(args) >= 2 {
		if b, err := os.ReadFile(args[len(args)-2]); err == nil {
			c.installed = string(b)
		}
	}
	f.calls = append(f.calls, c)
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(f.stdout))
	}
	if f.failOn[f.subcommand(c)] {
		return execx.Result{ExitCode: 1, Stderr: "boom"}, nil
	}
	return execx.Result{}, nil
}

// subcommand is the program run under `sudo -n [--]`, or the program itself.
func (f *fakeExec) subcommand(c execCall) string {
	if c.name != "sudo" {
		return c.name
	}
	rest := c.args
	if len(rest) > 0 && rest[0] == "-n" {
		rest = rest[1:]
	}
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return ""
	}
	return rest[0]
}

// words flattens a call to the argv sudo would execute, dropping `-n` and `--`.
func words(c execCall) []string {
	if c.name != "sudo" {
		return append([]string{c.name}, c.args...)
	}
	rest := c.args
	if len(rest) == 0 || rest[0] != "-n" {
		return append([]string{"sudo"}, rest...)
	}
	rest = rest[1:]
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	return append([]string{"sudo-n"}, rest...)
}

type fakeTerminal struct {
	cmds []*exec.Cmd
	err  error
}

func (f *fakeTerminal) Run(_ context.Context, cmd *exec.Cmd) error {
	f.cmds = append(f.cmds, cmd)
	return f.err
}

func newRunner(f *fakeExec) system.Runner {
	return system.Runner{Home: filepath.Join(os.TempDir(), "unused-home"), Exec: f.run}
}

func TestCommandStep_SudoUsesNonInteractiveSudoWithoutAShell(t *testing.T) {
	f := &fakeExec{}
	step := domain.CommandStep{Label: "upgrade", Argv: []string{"pacman", "-Syu", "--needed", "--noconfirm", "git"}, Sudo: true}
	res := newRunner(f).Run(context.Background(), step, nil)
	if res.Status != domain.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if len(f.calls) != 1 {
		t.Fatalf("calls = %+v, want one", f.calls)
	}
	want := []string{"-n", "--", "pacman", "-Syu", "--needed", "--noconfirm", "git"}
	if c := f.calls[0]; c.name != "sudo" || !reflect.DeepEqual(c.args, want) {
		t.Errorf("exec = %s %v, want sudo %v", c.name, c.args, want)
	}
}

func TestCommandStep_WithoutSudoRunsArgvDirectlyInDir(t *testing.T) {
	f := &fakeExec{}
	step := domain.CommandStep{Label: "build", Argv: []string{"makepkg", "-si", "--noconfirm"}, Dir: "/work/paru"}
	if res := newRunner(f).Run(context.Background(), step, nil); res.Status != domain.StepOK {
		t.Fatalf("result = %+v", res)
	}
	c := f.calls[0]
	if c.name != "makepkg" || !reflect.DeepEqual(c.args, []string{"-si", "--noconfirm"}) || c.dir != "/work/paru" {
		t.Errorf("exec = %+v", c)
	}
}

func TestCommandStep_OutputStreamsAsProgressLinesAndNonZeroExitFails(t *testing.T) {
	f := &fakeExec{stdout: "resolving dependencies\n", failOn: map[string]bool{"pacman": true}}
	var lines []string
	res := newRunner(f).Run(context.Background(), domain.CommandStep{Argv: []string{"pacman", "-Syu"}},
		func(p domain.StepProgress) { lines = append(lines, p.Line) })
	if res.Status != domain.StepFailed || res.Err == nil {
		t.Errorf("result = %+v, want failure", res)
	}
	if !reflect.DeepEqual(lines, []string{"resolving dependencies"}) {
		t.Errorf("progress lines = %q", lines)
	}
}

func TestCommandStep_PacmanAndParuRemovalIsRefusedBeforeAnyExec(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		sudo bool
	}{
		{"pacman -Rns", []string{"pacman", "-Rns", "htop"}, true},
		{"pacman -R", []string{"pacman", "-R", "htop"}, false},
		{"pacman --remove", []string{"pacman", "--remove", "htop"}, true},
		{"paru -Rns", []string{"paru", "-Rns", "htop"}, false},
		{"paru --remove", []string{"paru", "--remove", "htop"}, false},
		{"pacman --rem", []string{"pacman", "--rem", "htop"}, false},
		{"pacman --remo", []string{"pacman", "--remo", "htop"}, false},
		{"pacman -Rdd", []string{"pacman", "-Rdd", "htop"}, false},
		{"env pacman -R", []string{"env", "pacman", "-R", "x"}, false},
		{"sudo pacman -R", []string{"sudo", "pacman", "-R", "x"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeExec{}
			res := newRunner(f).Run(context.Background(), domain.CommandStep{Argv: tt.argv, Sudo: tt.sudo}, nil)
			if res.Status != domain.StepFailed || res.Err == nil {
				t.Errorf("result = %+v, want failure", res)
			}
			if len(f.calls) != 0 {
				t.Errorf("exec was called: %+v", f.calls)
			}
		})
	}
}

func TestCommandStep_InstallArgvIsNotMistakenForRemoval(t *testing.T) {
	f := &fakeExec{}
	for _, argv := range [][]string{
		{"pacman", "-Syu", "--needed", "--noconfirm", "ripgrep"},
		{"env", "pacman", "-S", "--refresh", "ripgrep"},
	} {
		step := domain.CommandStep{Argv: argv, Sudo: true}
		if res := newRunner(f).Run(context.Background(), step, nil); res.Status != domain.StepOK {
			t.Errorf("%v: result = %+v", argv, res)
		}
	}
}

func TestCommandStep_InteractiveGoesThroughTheTerminalPort(t *testing.T) {
	f, term := &fakeExec{}, &fakeTerminal{}
	r := newRunner(f)
	r.Terminal = term
	res := r.Run(context.Background(), domain.CommandStep{Label: "sudo", Argv: []string{"sudo", "-v"}, Interactive: true}, nil)
	if res.Status != domain.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if len(term.cmds) != 1 || !reflect.DeepEqual(term.cmds[0].Args, []string{"sudo", "-v"}) {
		t.Errorf("terminal cmds = %+v, want one `sudo -v`", term.cmds)
	}
	if len(f.calls) != 0 {
		t.Errorf("exec seam was used for the handoff: %+v", f.calls)
	}
}

func TestCommandStep_InteractiveFailureIsReported(t *testing.T) {
	boom := errors.New("exit status 1")
	r := newRunner(&fakeExec{})
	r.Terminal = &fakeTerminal{err: boom}
	res := r.Run(context.Background(), domain.CommandStep{Argv: []string{"sudo", "-v"}, Interactive: true}, nil)
	if res.Status != domain.StepFailed || !errors.Is(res.Err, boom) {
		t.Errorf("result = %+v, want failure wrapping the terminal error", res)
	}
}

func TestCommandStep_InteractiveWithoutTerminalFails(t *testing.T) {
	f := &fakeExec{}
	res := newRunner(f).Run(context.Background(), domain.CommandStep{Argv: []string{"sudo", "-v"}, Interactive: true}, nil)
	if res.Status != domain.StepFailed || res.Err == nil || !strings.Contains(res.Err.Error(), "interactive step needs a terminal") {
		t.Errorf("result = %+v, want %q", res, "interactive step needs a terminal")
	}
	if len(f.calls) != 0 {
		t.Errorf("exec was called: %+v", f.calls)
	}
}

const logindDropIn = "/etc/systemd/logind.conf.d/60-nuydeck-power-key.conf"

var modeArg = regexp.MustCompile(`^0?644$`)

func lstatResult(exists bool) func(string) (fs.FileInfo, error) {
	return func(path string) (fs.FileInfo, error) {
		if !exists {
			return nil, &fs.PathError{Op: "lstat", Path: path, Err: fs.ErrNotExist}
		}
		return os.Lstat(os.TempDir())
	}
}

func TestPrivilegedWriteFile_RunsBackupInstallThenAtomicMove(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprintf("target exists=%t", exists), func(t *testing.T) {
			f := &fakeExec{}
			r := newRunner(f)
			r.TempDir = t.TempDir()
			r.Lstat = lstatResult(exists)
			backup := logindDropIn + ".nuydeck-bak-20260101-000000"
			step := domain.WriteFileStep{Path: logindDropIn, Content: []byte("[Login]\nHandlePowerKey=ignore\n"), Mode: 0o644, Privileged: true, BackupTo: backup}

			if res := r.Run(context.Background(), step, nil); res.Status != domain.StepOK {
				t.Fatalf("result = %+v", res)
			}

			var got [][]string
			var install execCall
			for _, c := range f.calls {
				if c.name != "sudo" || len(c.args) == 0 || c.args[0] != "-n" {
					t.Errorf("call %s %v is not `sudo -n ...`", c.name, c.args)
				}
				w := words(c)
				got = append(got, w)
				if len(w) > 1 && w[1] == "install" {
					install = c
				}
			}
			wantCalls := 2
			if exists {
				wantCalls = 3
				if want := []string{"sudo-n", "cp", "-p", "--", logindDropIn, backup}; !reflect.DeepEqual(got[0], want) {
					t.Errorf("backup = %v, want %v", got[0], want)
				}
			}
			if len(got) != wantCalls {
				t.Fatalf("commands = %v, want %d", got, wantCalls)
			}
			inst, move := got[len(got)-2], got[len(got)-1]
			if len(inst) != 11 || inst[1] != "install" || inst[2] != "-m" || !modeArg.MatchString(inst[3]) ||
				!reflect.DeepEqual(inst[4:9], []string{"-o", "root", "-g", "root", "--"}) {
				t.Errorf("install = %v, want sudo-n install -m 644 -o root -g root -- <tmp> <dest>", inst)
			}
			if inst[len(inst)-1] != logindDropIn+".nuydeck-new" {
				t.Errorf("install target = %q, want %q", inst[len(inst)-1], logindDropIn+".nuydeck-new")
			}
			if wantMove := []string{"sudo-n", "mv", "-f", "--", logindDropIn + ".nuydeck-new", logindDropIn}; !reflect.DeepEqual(move, wantMove) {
				t.Errorf("move = %v, want %v", move, wantMove)
			}
			if install.installed != string(step.Content) {
				t.Errorf("temp file content = %q, want %q", install.installed, step.Content)
			}
		})
	}
}

func TestPrivilegedWriteFile_WithoutBackupPathNeverCopies(t *testing.T) {
	f := &fakeExec{}
	r := newRunner(f)
	r.TempDir = t.TempDir()
	step := domain.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}
	if res := r.Run(context.Background(), step, nil); res.Status != domain.StepOK {
		t.Fatalf("result = %+v", res)
	}
	for _, c := range f.calls {
		if f.subcommand(c) == "cp" {
			t.Errorf("unexpected backup call: %s %v", c.name, c.args)
		}
	}
}

func TestPrivilegedWriteFile_InstallFailureSkipsTheMoveAndRemovesTheStagedFile(t *testing.T) {
	f := &fakeExec{failOn: map[string]bool{"install": true}}
	r := newRunner(f)
	r.TempDir = t.TempDir()
	res := r.Run(context.Background(), domain.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != domain.StepFailed {
		t.Errorf("result = %+v, want failure", res)
	}
	for _, c := range f.calls {
		if f.subcommand(c) == "mv" {
			t.Errorf("mv ran after a failed install: %v", c.args)
		}
	}
	assertStagedRemoved(t, f)
}

func TestPrivilegedWriteFile_MoveFailureRemovesTheStagedFile(t *testing.T) {
	f := &fakeExec{failOn: map[string]bool{"mv": true}}
	r := newRunner(f)
	r.TempDir = t.TempDir()
	res := r.Run(context.Background(), domain.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != domain.StepFailed {
		t.Errorf("result = %+v, want failure", res)
	}
	assertStagedRemoved(t, f)
}

func assertStagedRemoved(t *testing.T, f *fakeExec) {
	t.Helper()
	want := []string{"sudo-n", "rm", "-f", "--", logindDropIn + ".nuydeck-new"}
	if last := words(f.calls[len(f.calls)-1]); !reflect.DeepEqual(last, want) {
		t.Errorf("last call = %v, want %v", last, want)
	}
}

func TestPrivilegedWriteFile_StagingDirIsCheckedBeforeAnyExec(t *testing.T) {
	loose := t.TempDir()
	if err := os.Chmod(loose, 0o777); err != nil {
		t.Fatal(err)
	}
	f := &fakeExec{}
	r := newRunner(f)
	r.TempDir = loose
	res := r.Run(context.Background(), domain.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != domain.StepFailed || res.Err == nil || !strings.Contains(res.Err.Error(), "writable by group or others") {
		t.Errorf("result = %+v, want a refused staging dir", res)
	}
	if len(f.calls) != 0 {
		t.Errorf("exec was called: %+v", f.calls)
	}
}

func TestPrivilegedWriteFile_DefaultStagingDirIsPrivateUnderHomeWhenNoRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	home := t.TempDir()
	f := &fakeExec{}
	r := system.Runner{Home: home, Exec: f.run}
	res := r.Run(context.Background(), domain.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != domain.StepOK {
		t.Fatalf("result = %+v", res)
	}
	dir := filepath.Join(home, ".cache", "nuydeck", "tmp")
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("staging dir = %v, %v, want mode 0700", info, err)
	}
	if got := f.calls[0].args; !strings.HasPrefix(got[len(got)-2], dir+string(filepath.Separator)) {
		t.Errorf("install source %q is not under %s", got[len(got)-2], dir)
	}
}

func TestPrivilegedWriteFile_RuntimeDirIsTheDefaultWhenSet(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	f := &fakeExec{}
	r := system.Runner{Home: t.TempDir(), Exec: f.run}
	res := r.Run(context.Background(), domain.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != domain.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if got := f.calls[0].args; !strings.HasPrefix(got[len(got)-2], runtimeDir+string(filepath.Separator)) {
		t.Errorf("install source %q is not under %s", got[len(got)-2], runtimeDir)
	}
}

func TestPrivilegedWriteFile_PathsOutsideAllowListAreRefusedBeforeAnyExec(t *testing.T) {
	paths := []string{
		"/etc/passwd",
		"/etc/sudoers",
		"/etc/pacman.conf.d/evil",
		"/etc/systemd/logind.conf.d/../../shadow",
		"/etc/systemd/system/evil.service",
		"relative/pacman.conf",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			f := &fakeExec{}
			r := newRunner(f)
			r.TempDir = t.TempDir()
			res := r.Run(context.Background(), domain.WriteFileStep{Path: p, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
			if res.Status != domain.StepFailed || res.Err == nil {
				t.Errorf("result = %+v, want failure", res)
			}
			if len(f.calls) != 0 {
				t.Errorf("exec was called: %+v", f.calls)
			}
		})
	}
}

func TestPrivilegedWriteFile_PacmanConfIsAllowed(t *testing.T) {
	f := &fakeExec{}
	r := newRunner(f)
	r.TempDir = t.TempDir()
	r.Lstat = lstatResult(false)
	res := r.Run(context.Background(), domain.WriteFileStep{Path: "/etc/pacman.conf", Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != domain.StepOK || len(f.calls) != 2 {
		t.Fatalf("result = %+v, calls = %+v", res, f.calls)
	}
	staged := "/etc/pacman.conf.nuydeck-new"
	if got := words(f.calls[0]); got[len(got)-1] != staged || got[1] != "install" {
		t.Errorf("install = %v, want it to stage %s", got, staged)
	}
	if want := []string{"sudo-n", "mv", "-f", "--", staged, "/etc/pacman.conf"}; !reflect.DeepEqual(words(f.calls[1]), want) {
		t.Errorf("move = %v, want %v", words(f.calls[1]), want)
	}
}

func TestAppendPackageStep_AppendsThroughTomlx(t *testing.T) {
	path := filepath.Join(t.TempDir(), "packages.toml")
	if err := os.WriteFile(path, []byte("version = 1\n\n[[package]]\nname = \"a\"\nsource = \"pacman\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := newRunner(&fakeExec{}).Run(context.Background(), domain.AppendPackageStep{Path: path, Entry: manifest.Package{Name: "htop", Source: manifest.SourceAUR}}, nil)
	if res.Status != domain.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), `name = "htop"`) {
		t.Errorf("file = %q, want htop appended", b)
	}
}

func TestFileStepsDelegateToFsx(t *testing.T) {
	dir := t.TempDir()
	r := newRunner(&fakeExec{})
	ctx := context.Background()

	steps := map[string]domain.Step{
		"theme":   domain.WriteFileStep{Path: filepath.Join(dir, "theme", "name"), Content: []byte("nuytek"), Mode: 0o644},
		"link":    domain.SymlinkStep{Path: filepath.Join(dir, "gpu_env.lua"), Target: "gpu_env_amd.lua"},
		"secrets": domain.WriteSecretsStep{Path: filepath.Join(dir, ".secrets")},
	}
	for name, s := range steps {
		if res := r.Run(ctx, s, nil); res.Status != domain.StepOK {
			t.Errorf("%s: %+v", name, res)
		}
	}
}
