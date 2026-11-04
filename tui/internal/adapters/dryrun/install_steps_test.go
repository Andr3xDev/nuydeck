package dryrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const sentinel = "SENTINEL-secret-value-4d2b"

func detailOf(t *testing.T, s domain.Step) string {
	t.Helper()
	res := Recorder{}.Run(context.Background(), s, nil)
	if res.Status != domain.StepWouldApply {
		t.Fatalf("%T: status = %v, want StepWouldApply", s, res.Status)
	}
	return res.Detail
}

func TestRecorderRendersInstallSteps(t *testing.T) {
	cases := []struct {
		name string
		step domain.Step
		want []string
	}{
		{"sudo command", domain.CommandStep{Label: "install pacman packages (2)", Sudo: true,
			Argv: []string{"pacman", "-Syu", "--needed", "--noconfirm", "git", "base-devel"}},
			[]string{"would run", "sudo pacman -Syu --needed --noconfirm git base-devel"}},
		{"plain command", domain.CommandStep{Argv: []string{"git", "clone", "--depth", "1", "https://example.org/x.git"}},
			[]string{"would run", "git clone --depth 1 https://example.org/x.git"}},
		{"reboot", domain.CommandStep{Label: "reboot", Sudo: true, Argv: []string{"systemctl", "reboot"}},
			[]string{"would run", "sudo systemctl reboot"}},
		{"privileged file with backup", domain.WriteFileStep{Path: "/etc/pacman.conf", Content: []byte("[options]\n"), Mode: 0o644, Privileged: true,
			BackupTo: "/etc/pacman.conf.nuydeck-bak-20260101-000000"},
			[]string{"would write", "/etc/pacman.conf", "/etc/pacman.conf.nuydeck-bak-20260101-000000"}},
		{"user file", domain.WriteFileStep{Path: "/home/u/.config/nuydeck/theme/name", Content: []byte("nuytek\n"), Mode: 0o644},
			[]string{"would write", "/home/u/.config/nuydeck/theme/name"}},
		{"symlink", domain.SymlinkStep{Path: "/home/u/.config/hypr/configs/gpu_env.lua", Target: "gpu_env_nvidia.lua"},
			[]string{"would link", "/home/u/.config/hypr/configs/gpu_env.lua", "gpu_env_nvidia.lua"}},
		{"append package", domain.AppendPackageStep{Path: "/repo/manifest/packages.toml", Entry: domain.Package{Name: "htop", Source: domain.SourceAUR}},
			[]string{"would append", "htop", "aur", "/repo/manifest/packages.toml"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := detailOf(t, c.step)
			if strings.HasPrefix(got, "would run domain.") {
				t.Fatalf("fell through to the generic fallback: %q", got)
			}
			for _, want := range c.want {
				if !strings.Contains(got, want) {
					t.Errorf("render = %q, want it to contain %q", got, want)
				}
			}
		})
	}
}

func TestRecorderSecretsStepListsNamesNeverValues(t *testing.T) {
	step := domain.WriteSecretsStep{Path: "/home/u/.secrets", Entries: []domain.SecretEntry{
		{Name: "A", Value: domain.Secret(sentinel + "-a")},
		{Name: "B", Value: domain.Secret(sentinel + "-b")},
	}}
	got := detailOf(t, step)
	for _, want := range []string{"would create", ".secrets", "with variables: A, B"} {
		if !strings.Contains(got, want) {
			t.Errorf("render = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, sentinel) {
		t.Errorf("render leaks a value: %q", got)
	}
}

func TestRecorderSecretsStepWithoutEntriesSaysEmptyFileMode600(t *testing.T) {
	got := detailOf(t, domain.WriteSecretsStep{Path: "/home/u/.secrets"})
	for _, want := range []string{"would create empty", ".secrets", "(mode 600)"} {
		if !strings.Contains(got, want) {
			t.Errorf("render = %q, want it to contain %q", got, want)
		}
	}
}

func TestRecorderSaveStatePrintsIgnoredPackages(t *testing.T) {
	got := detailOf(t, domain.SaveStateStep{State: domain.State{IgnoredPackages: []string{"htop", "tmux"}}})
	if !strings.HasPrefix(got, "would save state") {
		t.Errorf("render = %q, want prefix %q", got, "would save state")
	}
	for _, want := range []string{"ignored_packages", "htop", "tmux"} {
		if !strings.Contains(got, want) {
			t.Errorf("render = %q, want it to contain %q", got, want)
		}
	}
}

func TestRecorderInstallStepsTouchNoFileAndLeakNoSentinel(t *testing.T) {
	dir := t.TempDir()
	steps := []domain.Step{
		domain.WriteFileStep{Path: filepath.Join(dir, "theme", "name"), Content: []byte("nuytek"), Mode: 0o644},
		domain.SymlinkStep{Path: filepath.Join(dir, "gpu_env.lua"), Target: "gpu_env_amd.lua"},
		domain.WriteSecretsStep{Path: filepath.Join(dir, ".secrets"), Entries: []domain.SecretEntry{{Name: "A", Value: sentinel}}},
		domain.AppendPackageStep{Path: filepath.Join(dir, "packages.toml"), Entry: domain.Package{Name: "htop", Source: domain.SourcePacman}},
	}
	for _, s := range steps {
		res := Recorder{}.Run(context.Background(), s, nil)
		for _, format := range []string{"%v", "%+v", "%#v"} {
			if out := fmt.Sprintf(format, res); strings.Contains(out, sentinel) {
				t.Errorf("%T result printed with %s leaks the value: %s", s, format, out)
			}
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("recorder created files: %v", entries)
	}
}
