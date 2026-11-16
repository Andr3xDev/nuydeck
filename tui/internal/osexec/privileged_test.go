package osexec_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/osexec"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

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
			step := plan.WriteFileStep{Path: logindDropIn, Content: []byte("[Login]\nHandlePowerKey=ignore\n"), Mode: 0o644, Privileged: true, BackupTo: backup}

			if res := r.Run(context.Background(), step, nil); res.Status != plan.StepOK {
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
	step := plan.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}
	if res := r.Run(context.Background(), step, nil); res.Status != plan.StepOK {
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
	res := r.Run(context.Background(), plan.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != plan.StepFailed {
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
	res := r.Run(context.Background(), plan.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != plan.StepFailed {
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
	res := r.Run(context.Background(), plan.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != plan.StepFailed || res.Err == nil || !strings.Contains(res.Err.Error(), "writable by group or others") {
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
	r := osexec.Runner{Home: home, Exec: f.run}
	res := r.Run(context.Background(), plan.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != plan.StepOK {
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
	r := osexec.Runner{Home: t.TempDir(), Exec: f.run}
	res := r.Run(context.Background(), plan.WriteFileStep{Path: logindDropIn, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != plan.StepOK {
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
			res := r.Run(context.Background(), plan.WriteFileStep{Path: p, Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
			if res.Status != plan.StepFailed || res.Err == nil {
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
	res := r.Run(context.Background(), plan.WriteFileStep{Path: "/etc/pacman.conf", Content: []byte("x"), Mode: 0o644, Privileged: true}, nil)
	if res.Status != plan.StepOK || len(f.calls) != 2 {
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
