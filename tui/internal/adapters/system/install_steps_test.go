package system_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/system"
	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
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
	step := plan.CommandStep{Label: "upgrade", Argv: []string{"pacman", "-Syu", "--needed", "--noconfirm", "git"}, Sudo: true}
	res := newRunner(f).Run(context.Background(), step, nil)
	if res.Status != plan.StepOK {
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
	step := plan.CommandStep{Label: "build", Argv: []string{"makepkg", "-si", "--noconfirm"}, Dir: "/work/paru"}
	if res := newRunner(f).Run(context.Background(), step, nil); res.Status != plan.StepOK {
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
	res := newRunner(f).Run(context.Background(), plan.CommandStep{Argv: []string{"pacman", "-Syu"}},
		func(p plan.StepProgress) { lines = append(lines, p.Line) })
	if res.Status != plan.StepFailed || res.Err == nil {
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
			res := newRunner(f).Run(context.Background(), plan.CommandStep{Argv: tt.argv, Sudo: tt.sudo}, nil)
			if res.Status != plan.StepFailed || res.Err == nil {
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
		step := plan.CommandStep{Argv: argv, Sudo: true}
		if res := newRunner(f).Run(context.Background(), step, nil); res.Status != plan.StepOK {
			t.Errorf("%v: result = %+v", argv, res)
		}
	}
}

func TestCommandStep_InteractiveGoesThroughTheTerminalPort(t *testing.T) {
	f, term := &fakeExec{}, &fakeTerminal{}
	r := newRunner(f)
	r.Terminal = term
	res := r.Run(context.Background(), plan.CommandStep{Label: "sudo", Argv: []string{"sudo", "-v"}, Interactive: true}, nil)
	if res.Status != plan.StepOK {
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
	res := r.Run(context.Background(), plan.CommandStep{Argv: []string{"sudo", "-v"}, Interactive: true}, nil)
	if res.Status != plan.StepFailed || !errors.Is(res.Err, boom) {
		t.Errorf("result = %+v, want failure wrapping the terminal error", res)
	}
}

func TestCommandStep_InteractiveWithoutTerminalFails(t *testing.T) {
	f := &fakeExec{}
	res := newRunner(f).Run(context.Background(), plan.CommandStep{Argv: []string{"sudo", "-v"}, Interactive: true}, nil)
	if res.Status != plan.StepFailed || res.Err == nil || !strings.Contains(res.Err.Error(), "interactive step needs a terminal") {
		t.Errorf("result = %+v, want %q", res, "interactive step needs a terminal")
	}
	if len(f.calls) != 0 {
		t.Errorf("exec was called: %+v", f.calls)
	}
}
