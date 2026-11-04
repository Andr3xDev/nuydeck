package sysx_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/sysx"
)

type call struct {
	name string
	args []string
}

type fakeRun struct {
	calls  []call
	stdout string
	result execx.Result
	err    error
}

func (f *fakeRun) run(_ context.Context, out io.Writer, name string, args ...string) (execx.Result, error) {
	f.calls = append(f.calls, call{name, args})
	if out != nil {
		_, _ = io.WriteString(out, f.stdout)
	}
	return f.result, f.err
}

func (f *fakeRun) onlyCall(t *testing.T, want call) {
	t.Helper()
	if len(f.calls) != 1 || f.calls[0].name != want.name || !reflect.DeepEqual(f.calls[0].args, want.args) {
		t.Errorf("calls = %+v, want exactly %+v", f.calls, want)
	}
}

const lspciNvidia = "Slot:\t01:00.0\nClass:\tVGA compatible controller [0300]\nVendor:\tNVIDIA Corporation [10de]\nDevice:\tGA106 [2503]\n\n"

func TestDetectGPUs_RunsLspciAndParsesDevices(t *testing.T) {
	f := &fakeRun{stdout: lspciNvidia}
	devices, err := sysx.Probe{Run: f.run}.DetectGPUs(context.Background())
	if err != nil {
		t.Fatalf("DetectGPUs() error = %v", err)
	}
	f.onlyCall(t, call{"lspci", []string{"-vmmnn"}})
	if len(devices) != 1 || devices[0].VendorID != "10de" || devices[0].ClassID != "0300" {
		t.Errorf("devices = %+v, want one NVIDIA VGA device", devices)
	}
}

func TestDetectGPUs_MissingBinaryKeepsErrNotFound(t *testing.T) {
	f := &fakeRun{err: &exec.Error{Name: "lspci", Err: exec.ErrNotFound}}
	_, err := sysx.Probe{Run: f.run}.DetectGPUs(context.Background())
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("error = %v, want exec.ErrNotFound", err)
	}
}

func TestDetectGPUs_NonZeroExitIsAnError(t *testing.T) {
	f := &fakeRun{result: execx.Result{ExitCode: 1, Stderr: "lspci: boom"}}
	if _, err := (sysx.Probe{Run: f.run}).DetectGPUs(context.Background()); err == nil {
		t.Error("error = nil, want error")
	}
}

func TestInstalledPackages_ParsesPacmanQq(t *testing.T) {
	f := &fakeRun{stdout: "bash\nhtop\n"}
	got, err := sysx.Probe{Run: f.run}.InstalledPackages(context.Background())
	if err != nil {
		t.Fatalf("InstalledPackages() error = %v", err)
	}
	f.onlyCall(t, call{"pacman", []string{"-Qq"}})
	if len(got) != 2 || !got["bash"] || !got["htop"] {
		t.Errorf("got %v, want bash and htop", got)
	}
}

func TestInstalledPackages_NonZeroExitIsAnError(t *testing.T) {
	f := &fakeRun{result: execx.Result{ExitCode: 1}}
	if _, err := (sysx.Probe{Run: f.run}).InstalledPackages(context.Background()); err == nil {
		t.Error("error = nil, want error")
	}
}

func TestExplicitPackages_ParsesPacmanQqe(t *testing.T) {
	f := &fakeRun{stdout: "bash\nhtop\n"}
	got, err := sysx.Probe{Run: f.run}.ExplicitPackages(context.Background())
	if err != nil {
		t.Fatalf("ExplicitPackages() error = %v", err)
	}
	f.onlyCall(t, call{"pacman", []string{"-Qqe"}})
	if !reflect.DeepEqual(got, []string{"bash", "htop"}) {
		t.Errorf("got %v, want [bash htop]", got)
	}
}

func TestExplicitPackages_FailureReturnsErrorAndNoList(t *testing.T) {
	f := &fakeRun{stdout: "partial\n", result: execx.Result{ExitCode: 1, Stderr: "error: failed to initialize alpm library"}}
	got, err := sysx.Probe{Run: f.run}.ExplicitPackages(context.Background())
	if err == nil || len(got) != 0 {
		t.Errorf("got %v, %v; want an error and no list", got, err)
	}
}

func TestLoginShell_ReadsField7OfGetentPasswd(t *testing.T) {
	f := &fakeRun{stdout: "alice:x:1000:1000:Alice:/home/alice:/usr/bin/zsh\n"}
	got, err := sysx.Probe{Run: f.run, User: "alice"}.LoginShell(context.Background())
	if err != nil {
		t.Fatalf("LoginShell() error = %v", err)
	}
	f.onlyCall(t, call{"getent", []string{"passwd", "alice"}})
	if got != "/usr/bin/zsh" {
		t.Errorf("shell = %q, want /usr/bin/zsh", got)
	}
}

func TestLoginShell_MalformedOrFailedLookupIsAnError(t *testing.T) {
	tests := []struct {
		name string
		f    *fakeRun
	}{
		{"too few fields", &fakeRun{stdout: "alice:x:1000\n"}},
		{"user not found", &fakeRun{result: execx.Result{ExitCode: 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := (sysx.Probe{Run: tt.f.run, User: "alice"}).LoginShell(context.Background()); err == nil {
				t.Error("error = nil, want error")
			}
		})
	}
}

func TestReadSystemFile_RefusesPathsOutsideAllowList(t *testing.T) {
	paths := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/pacman.conf.d/x",
		"/etc/pacman.conf/../shadow",
		"/etc/systemd/logind.conf.d/../../shadow",
		"/etc/systemd/logind.conf.d/sub/x.conf",
		"/etc/systemd/logind.conf.d",
		"pacman.conf",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			_, err := sysx.Probe{}.ReadSystemFile(p)
			if !errors.Is(err, sysx.ErrPathNotAllowed) {
				t.Errorf("error = %v, want ErrPathNotAllowed", err)
			}
		})
	}
}

func TestReadSystemFile_AllowedDropInPathIsReadNotRefused(t *testing.T) {
	_, err := sysx.Probe{}.ReadSystemFile("/etc/systemd/logind.conf.d/zz-nuydeck-test-absent.conf")
	if errors.Is(err, sysx.ErrPathNotAllowed) || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want fs.ErrNotExist from an actual read", err)
	}
}

func TestProbeCommandsAreReadOnly(t *testing.T) {
	f := &fakeRun{stdout: lspciNvidia}
	p := sysx.Probe{Run: f.run, User: "alice"}
	ctx := context.Background()
	_, _ = p.DetectGPUs(ctx)
	_, _ = p.InstalledPackages(ctx)
	_, _ = p.ExplicitPackages(ctx)
	_, _ = p.LoginShell(ctx)
	for _, c := range f.calls {
		joined := c.name + " " + strings.Join(c.args, " ")
		switch joined {
		case "lspci -vmmnn", "pacman -Qq", "pacman -Qqe", "getent passwd alice":
		default:
			t.Errorf("unexpected command %q", joined)
		}
	}
	if len(f.calls) != 4 {
		t.Errorf("calls = %d, want 4", len(f.calls))
	}
}
