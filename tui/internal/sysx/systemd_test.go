package sysx_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
	"github.com/Andr3xDev/nuydeck/tui/internal/sysx"
)

var _ services.StateProbe = sysx.SystemctlProbe{}

const showOutput = "Id=a.service\nLoadState=loaded\nUnitFileState=enabled\nActiveState=active\n\n" +
	"Id=b.service\nLoadState=not-found\nUnitFileState=\nActiveState=inactive\n"

var showArgs = []string{"show", "--no-pager", "-p", "Id,LoadState,UnitFileState,ActiveState"}

// userBus points XDG_RUNTIME_DIR at a short temp dir holding a real unix socket named bus.
func userBus(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	ln, err := net.Listen("unix", filepath.Join(dir, "bus"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	t.Setenv("XDG_RUNTIME_DIR", dir)
}

func TestSystemctlProbe_SystemScopeRunsOneShowWithoutSudo(t *testing.T) {
	f := &fakeRun{stdout: showOutput}
	got, err := sysx.SystemctlProbe{Run: f.run}.Read(context.Background(), plan.ScopeSystem, []string{"a.service", "b.service"})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	f.onlyCall(t, call{"systemctl", append(append([]string(nil), showArgs...), "a.service", "b.service")})
	if !got["a.service"].Found || got["a.service"].Enabled != services.EnabledEnabled || got["b.service"].Found {
		t.Errorf("states = %+v", got)
	}
}

func TestSystemctlProbe_AliasIdIsReadUnderTheRequestedName(t *testing.T) {
	f := &fakeRun{stdout: "Id=real.service\nLoadState=loaded\nUnitFileState=enabled\nActiveState=active\n"}

	got, err := sysx.SystemctlProbe{Run: f.run}.Read(context.Background(), plan.ScopeSystem, []string{"alias.service"})

	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if s := got["alias.service"]; !s.Found || s.Active != services.ActiveActive {
		t.Errorf("states = %+v, want alias.service found and active", got)
	}
}

func TestSystemctlProbe_UserScopeWithBusUsesUserFlag(t *testing.T) {
	userBus(t)
	f := &fakeRun{stdout: showOutput}
	if _, err := (sysx.SystemctlProbe{Run: f.run}).Read(context.Background(), plan.ScopeUser, []string{"a.service"}); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	f.onlyCall(t, call{"systemctl", append([]string{"--user"}, append(append([]string(nil), showArgs...), "a.service")...)})
}

func TestSystemctlProbe_UserScopeWithoutBusReturnsErrNoUserBusAndSpawnsNothing(t *testing.T) {
	empty := t.TempDir()
	for name, runtimeDir := range map[string]string{"unset": "", "no bus socket": empty} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
			f := &fakeRun{stdout: showOutput}
			_, err := sysx.SystemctlProbe{Run: f.run}.Read(context.Background(), plan.ScopeUser, []string{"a.service"})
			if !errors.Is(err, services.ErrNoUserBus) || len(f.calls) != 0 {
				t.Errorf("err = %v, calls = %+v, want ErrNoUserBus and no spawn", err, f.calls)
			}
		})
	}
}

func TestSystemctlProbe_UserFailureReportsFirstStderrLineNotNoUserBus(t *testing.T) {
	userBus(t)
	f := &fakeRun{result: execx.Result{ExitCode: 1, Stderr: "Failed to connect to bus\nsecond line\n"}}
	_, err := sysx.SystemctlProbe{Run: f.run}.Read(context.Background(), plan.ScopeUser, []string{"a.service"})
	if err == nil || errors.Is(err, services.ErrNoUserBus) {
		t.Fatalf("err = %v, want a non-ErrNoUserBus error", err)
	}
	if !strings.Contains(err.Error(), "Failed to connect to bus") || strings.Contains(err.Error(), "second line") {
		t.Errorf("err = %q, want first stderr line only", err)
	}
}
