package install

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

type planBuilder struct{ plan InstallPlan }

func (b *planBuilder) add(phase Phase, step plan.Step, skip string) {
	b.plan.Steps = append(b.plan.Steps, PlannedStep{Phase: phase, Step: step, Skip: skip})
}

// Plan reads the machine and builds the ordered steps for the draft; it mutates nothing.
func (i *Installer) Plan(ctx context.Context, d Draft) (InstallPlan, error) {
	installed, err := i.Probe.InstalledPackages(ctx)
	if err != nil {
		return InstallPlan{}, fmt.Errorf("installed packages: %w", err)
	}
	pacman, aur := i.selectedPackages(d)

	b := &planBuilder{}
	for _, s := range plan.PrivilegeSteps() {
		b.add(PhasePrivilege, s, "")
	}

	i.planMultilib(b)
	if len(pacman) > 0 {
		argv := slices.Concat(pacmanInstallArgs, pacman)
		b.add(PhasePacman, plan.CommandStep{Label: fmt.Sprintf("install pacman packages (%d)", len(pacman)), Argv: argv, Sudo: true}, skipIfInstalled(pacman, installed))
	}
	if err := i.planAUR(b, aur, installed); err != nil {
		return InstallPlan{}, err
	}
	i.planLogind(b)
	if err := i.planSync(ctx, b); err != nil {
		return InstallPlan{}, err
	}
	if err := i.planEnv(b, d); err != nil {
		return InstallPlan{}, err
	}
	if err := i.planShell(ctx, b); err != nil {
		return InstallPlan{}, err
	}
	i.planServices(b, d)
	i.planTheme(b, d)

	b.add(PhaseRecord, plan.AppendLogStep{Entry: state.LogEntry{Op: "install"}}, "")
	b.add(PhaseReboot, plan.CommandStep{Label: "reboot", Argv: []string{"systemctl", "reboot"}, Sudo: true}, "")
	return b.plan, nil
}

func (d Draft) includes(p manifest.Package) bool {
	if d.Profile == ProfileCustom {
		return slices.Contains(d.Packages, p.Name)
	}
	return slices.Contains(p.Profiles, d.Profile)
}

func (i *Installer) selectedDrivers(d Draft) []manifest.Driver {
	return slices.DeleteFunc(slices.Clone(i.Drivers), func(dr manifest.Driver) bool { return !slices.Contains(d.Drivers, dr.ID) })
}

// selectedPackages returns the sorted pacman and AUR names; driver packages and the paru prerequisites go to pacman.
func (i *Installer) selectedPackages(d Draft) (pacman, aur []string) {
	for _, p := range i.Catalog {
		switch {
		case !d.includes(p):
		case p.Source == manifest.SourceAUR:
			aur = append(aur, p.Name)
		default:
			pacman = append(pacman, p.Name)
		}
	}
	for _, dr := range i.selectedDrivers(d) {
		pacman = append(pacman, dr.Packages...)
	}
	if len(aur) > 0 {
		pacman = append(pacman, manifest.ParuPrerequisites...)
	}
	slices.Sort(pacman)
	slices.Sort(aur)
	return slices.Compact(pacman), aur
}

func skipIfInstalled(names []string, installed map[string]bool) string {
	for _, n := range names {
		if !installed[n] {
			return ""
		}
	}
	return skipInstalled
}

// readSystemFile returns nil content for a missing file; any other failure blocks the plan.
func (i *Installer) readSystemFile(b *planBuilder, file string) ([]byte, bool) {
	content, err := i.Probe.ReadSystemFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		b.plan.Blocking = append(b.plan.Blocking, fmt.Errorf("read %s: %w", file, err))
		return nil, false
	}
	return content, true
}

func (i *Installer) backupSuffix() string {
	return ".nuydeck-bak-" + i.Now().UTC().Format(backupTimestampLayout)
}
