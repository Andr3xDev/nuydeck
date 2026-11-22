package packages

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func row(name string, status Status, source manifest.Source) Row {
	return Row{Name: name, Group: "g", Source: source, Status: status}
}

// pacmanCommand splits a plan into its sudo priming and its single privileged command.
func pacmanCommand(t *testing.T, p plan.Plan) plan.CommandStep {
	t.Helper()
	priming := plan.PrivilegeSteps()
	if len(p.Steps) != len(priming)+1 {
		t.Fatalf("steps = %+v, want the sudo priming then one command", p.Steps)
	}
	if !reflect.DeepEqual(p.Steps[:len(priming)], priming) {
		t.Errorf("leading steps = %+v, want plan.PrivilegeSteps()", p.Steps[:len(priming)])
	}
	cmd, ok := p.Steps[len(priming)].(plan.CommandStep)
	if !ok {
		t.Fatalf("last step = %T, want plan.CommandStep", p.Steps[len(priming)])
	}
	if !cmd.Sudo {
		t.Errorf("command = %+v, want Sudo", cmd)
	}
	return cmd
}

func TestPlanInstall(t *testing.T) {
	tests := []struct {
		name        string
		rows        []Row
		wantArgv    []string // nil means an empty plan
		wantNotices []string
		noticeAbout string // a name that exactly one notice must mention
	}{
		{
			name:     "installs the missing rows only",
			rows:     []Row{row("a", "missing", manifest.SourcePacman), row("b", "installed", manifest.SourcePacman), row("c", "drift", ""), row("d", "missing", manifest.SourcePacman)},
			wantArgv: []string{"pacman", "-S", "--needed", "--noconfirm", "--", "a", "d"},
		},
		{
			name:        "drops a missing AUR package with one notice",
			rows:        []Row{row("a", "missing", manifest.SourcePacman), row("paru-bin", "missing", manifest.SourceAUR)},
			wantArgv:    []string{"pacman", "-S", "--needed", "--noconfirm", "--", "a"},
			noticeAbout: "paru-bin",
		},
		{
			name:        "nothing missing gives an empty plan and a notice",
			rows:        []Row{row("b", "installed", manifest.SourcePacman), row("c", "drift", "")},
			wantNotices: []string{"no marked package is missing"},
		},
		{
			name:        "only AUR packages missing gives an empty plan",
			rows:        []Row{row("paru-bin", "missing", manifest.SourceAUR)},
			noticeAbout: "paru-bin",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, notices := PlanInstall(tt.rows)

			if tt.wantArgv == nil {
				if !got.Empty() {
					t.Errorf("plan = %+v, want empty", got)
				}
			} else if cmd := pacmanCommand(t, got); !slices.Equal(cmd.Argv, tt.wantArgv) {
				t.Errorf("argv = %v, want %v", cmd.Argv, tt.wantArgv)
			}
			for _, want := range tt.wantNotices {
				if !slices.Contains(notices, want) {
					t.Errorf("notices = %q, want %q among them", notices, want)
				}
			}
			if tt.noticeAbout != "" {
				mentions := 0
				for _, n := range notices {
					if strings.Contains(n, tt.noticeAbout) {
						mentions++
					}
				}
				if mentions != 1 {
					t.Errorf("notices = %q, want exactly one mentioning %s", notices, tt.noticeAbout)
				}
			}
		})
	}
}

func TestPlanUninstall(t *testing.T) {
	rows := []Row{row("a", "installed", manifest.SourcePacman), row("b", "missing", manifest.SourcePacman), row("c", "drift", ""), row("d", "installed", manifest.SourceAUR)}

	got, _ := PlanUninstall(rows)

	cmd := pacmanCommand(t, got)
	if want := []string{"pacman", "-Rns", "--noconfirm", "--", "a", "d"}; !slices.Equal(cmd.Argv, want) {
		t.Errorf("argv = %v, want %v", cmd.Argv, want)
	}
	if got.ExtraConfirm == "" {
		t.Error("ExtraConfirm is empty, want a second confirm for a destructive action")
	}
	if !strings.Contains(strings.ToLower(got.Confirm), "manifest") {
		t.Errorf("Confirm = %q, want it to say the manifest is not edited", got.Confirm)
	}
}

func TestPlanUninstall_NothingInstalledGivesAnEmptyPlanAndANotice(t *testing.T) {
	got, notices := PlanUninstall([]Row{row("b", "missing", manifest.SourcePacman), row("c", "drift", "")})

	if !got.Empty() {
		t.Errorf("plan = %+v, want empty", got)
	}
	if !slices.Contains(notices, "no marked package is installed") {
		t.Errorf("notices = %q, want the nothing-installed notice", notices)
	}
}
