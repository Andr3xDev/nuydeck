package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const (
	pacmanConfFile         = "/etc/pacman.conf"
	logindDropInFile       = "/etc/systemd/logind.conf.d/60-nuydeck-power-key.conf"
	displayManagerUnitLink = "/etc/systemd/system/display-manager.service"
	lyUnit                 = "ly.service"

	// logindDropIn: a short power-button press is ignored, a long press powers off.
	logindDropIn = "[Login]\nHandlePowerKey=ignore\nHandlePowerKeyLongPress=poweroff\n"

	backupTimestampLayout = "20060102-150405"
	installedFileMode     = 0o644

	paruRepoURL     = "https://aur.archlinux.org/paru.git"
	ohMyZshURL      = "https://github.com/ohmyzsh/ohmyzsh.git"
	zshHighlightURL = "https://github.com/zsh-users/zsh-syntax-highlighting.git"
	zshSuggestURL   = "https://github.com/zsh-users/zsh-autosuggestions.git"
	zshBinary       = "/usr/bin/zsh"

	themeName       = "nuytek"
	defaultVariant  = "nadir"
	gpuEnvLinkName  = "gpu_env.lua"
	repoGPUEnvDir   = "config/hypr/configs"
	homeGPUEnvDir   = ".config/hypr/configs"
	themeFilesDir   = ".config/nuydeck/theme"
	paruBootstrapAt = ".cache/nuydeck/paru-bootstrap"
)

const skipInstalled = "already installed"

var pacmanInstallArgs = []string{"pacman", "-Syu", "--needed", "--noconfirm"}

var aurInstallArgs = []string{"paru", "-S", "--needed", "--noconfirm", "--skipreview", "--sudoflags", "-n"}

// sudoProbeStep checks the sudo timestamp without ever prompting.
func sudoProbeStep(label string) domain.CommandStep {
	return domain.CommandStep{Label: label, Argv: []string{"sudo", "-n", "-v"}, Quiet: true}
}

type planBuilder struct{ plan InstallPlan }

func (b *planBuilder) add(phase domain.Phase, step domain.Step, skip string) {
	b.plan.Steps = append(b.plan.Steps, domain.PlannedStep{Phase: phase, Step: step, Skip: skip})
}

// Plan reads the machine and builds the ordered steps for the draft; it mutates nothing.
func (i *Installer) Plan(ctx context.Context, d Draft) (InstallPlan, error) {
	installed, err := i.Probe.InstalledPackages(ctx)
	if err != nil {
		return InstallPlan{}, fmt.Errorf("installed packages: %w", err)
	}
	pacman, aur := i.selectedPackages(d)

	b := &planBuilder{}
	b.add(domain.PhasePrivilege, domain.CommandStep{Label: "authenticate sudo", Argv: []string{"sudo", "-v"}, Interactive: true}, "")
	b.add(domain.PhasePrivilege, sudoProbeStep("verify sudo credentials"), "")

	i.planMultilib(b)
	if len(pacman) > 0 {
		argv := slices.Concat(pacmanInstallArgs, pacman)
		b.add(domain.PhasePacman, domain.CommandStep{Label: fmt.Sprintf("install pacman packages (%d)", len(pacman)), Argv: argv, Sudo: true}, skipIfInstalled(pacman, installed))
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

	b.add(domain.PhaseRecord, domain.AppendLogStep{Entry: state.LogEntry{Op: "install"}}, "")
	b.add(domain.PhaseReboot, domain.CommandStep{Label: "reboot", Argv: []string{"systemctl", "reboot"}, Sudo: true}, "")
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

func (i *Installer) planMultilib(b *planBuilder) {
	content, ok := i.readSystemFile(b, pacmanConfFile)
	if !ok {
		return
	}
	enabled, changed, err := domain.EnableMultilib(content)
	if err != nil {
		b.plan.Blocking = append(b.plan.Blocking, err)
		return
	}
	step := domain.WriteFileStep{Path: pacmanConfFile, Content: enabled, Mode: installedFileMode, Privileged: true, BackupTo: pacmanConfFile + i.backupSuffix()}
	skip := ""
	if !changed {
		skip = "multilib already active"
	}
	b.add(domain.PhasePacmanConf, step, skip)
}

func (i *Installer) planAUR(b *planBuilder, aur []string, installed map[string]bool) error {
	if len(aur) == 0 {
		return nil
	}
	cacheDir := path.Join(i.Home, paruBootstrapAt)
	paruSkip := ""
	if i.Probe.HasCommand("paru") {
		paruSkip = "paru already installed"
	}
	cloneSkip := paruSkip
	if cloneSkip == "" {
		kind, err := i.Service.Inspector.PathKind(cacheDir)
		if err != nil {
			return err
		}
		if kind != domain.KindNone {
			cloneSkip = "clone already present"
		}
	}
	b.add(domain.PhaseParuBootstrap, domain.CommandStep{Label: "clone paru", Argv: []string{"git", "clone", paruRepoURL, cacheDir}}, cloneSkip)
	b.add(domain.PhaseParuBootstrap, domain.CommandStep{Label: "build and install paru", Argv: []string{"makepkg", "-si", "--noconfirm"}, Dir: cacheDir, UsesSudo: true}, paruSkip)
	b.add(domain.PhaseAur, domain.CommandStep{Label: fmt.Sprintf("install AUR packages (%d)", len(aur)), Argv: slices.Concat(aurInstallArgs, aur), UsesSudo: true}, skipIfInstalled(aur, installed))
	return nil
}

func (i *Installer) planLogind(b *planBuilder) {
	existing, ok := i.readSystemFile(b, logindDropInFile)
	if !ok {
		return
	}
	step := domain.WriteFileStep{Path: logindDropInFile, Content: []byte(logindDropIn), Mode: installedFileMode, Privileged: true}
	skip := ""
	switch {
	case string(existing) == logindDropIn:
		skip = "drop-in already in place"
	case len(existing) > 0:
		step.BackupTo = logindDropInFile + i.backupSuffix()
	}
	b.add(domain.PhaseSystem, step, skip)
}

func (i *Installer) planSync(ctx context.Context, b *planBuilder) error {
	var fonts, configs []manifest.ResolvedEntry
	for _, e := range i.Entries {
		if e.Root == manifest.RootFonts {
			fonts = append(fonts, e)
		} else {
			configs = append(configs, e)
		}
	}
	for _, group := range []struct {
		phase   domain.Phase
		entries []manifest.ResolvedEntry
	}{{domain.PhaseConfigs, configs}, {domain.PhaseFonts, fonts}} {
		steps, notices, blocking, err := i.pushSteps(ctx, group.entries)
		if err != nil {
			return err
		}
		for _, s := range steps {
			b.add(group.phase, s, "")
		}
		b.plan.Notices = append(b.plan.Notices, notices...)
		b.plan.Blocking = append(b.plan.Blocking, blocking...)
		if group.phase == domain.PhaseFonts {
			skip := ""
			if len(steps) == 0 {
				skip = "no fonts to copy"
			}
			b.add(domain.PhaseFonts, domain.CommandStep{Label: "refresh font cache", Argv: []string{"fc-cache", "-f"}}, skip)
		}
	}
	return nil
}

func (i *Installer) planEnv(b *planBuilder, d Draft) error {
	if err := i.planGPUEnvLink(b, d); err != nil {
		return err
	}
	if d.SkipSecrets {
		return nil
	}
	secrets := path.Join(i.Home, ".secrets")
	kind, err := i.Service.Inspector.PathKind(secrets)
	if err != nil {
		return err
	}
	skip := ""
	if kind != domain.KindNone {
		skip = "left as is"
	}
	step := domain.WriteSecretsStep{Path: secrets}
	for _, e := range d.Secrets {
		if e.Value != "" {
			step.Entries = append(step.Entries, e)
		}
	}
	b.add(domain.PhaseEnv, step, skip)
	return nil
}

func (i *Installer) planGPUEnvLink(b *planBuilder, d Draft) error {
	var ids []string
	for _, dr := range i.selectedDrivers(d) {
		ids = append(ids, dr.ID)
	}
	target := domain.GPUEnvFileName(ids)
	if target == "" {
		return nil
	}
	link := domain.SymlinkStep{Path: path.Join(i.Home, homeGPUEnvDir, gpuEnvLinkName), Target: target}
	if d.SkipGPUEnvLink {
		b.add(domain.PhaseEnv, link, "GPU env link skipped by owner")
		return nil
	}
	in := i.Service.Inspector
	repoKind, err := in.PathKind(path.Join(i.Service.RepoDir, repoGPUEnvDir, target))
	if err != nil {
		return err
	}
	if repoKind == domain.KindNone {
		b.plan.Notices = append(b.plan.Notices, fmt.Sprintf("%s not linked: %s is missing from the repo", gpuEnvLinkName, target))
		return nil
	}
	kind, err := in.PathKind(link.Path)
	if err != nil {
		return err
	}
	switch kind {
	case domain.KindNone:
		b.add(domain.PhaseEnv, link, "")
	case domain.KindLink:
		current, _ := in.LinkTarget(link.Path)
		skip := ""
		if current == target {
			skip = "already linked"
		}
		b.add(domain.PhaseEnv, link, skip)
	default:
		b.plan.Notices = append(b.plan.Notices, fmt.Sprintf("%s is a regular file and was left as is", link.Path))
		b.add(domain.PhaseEnv, link, "regular file kept")
	}
	return nil
}

func (i *Installer) planShell(ctx context.Context, b *planBuilder) error {
	ohMyZsh := path.Join(i.Home, ".oh-my-zsh")
	plugins := path.Join(ohMyZsh, "custom/plugins")
	for _, c := range []struct{ url, dest string }{
		{ohMyZshURL, ohMyZsh},
		{zshHighlightURL, path.Join(plugins, "zsh-syntax-highlighting")},
		{zshSuggestURL, path.Join(plugins, "zsh-autosuggestions")},
	} {
		kind, err := i.Service.Inspector.PathKind(c.dest)
		if err != nil {
			return err
		}
		skip := ""
		if kind != domain.KindNone {
			skip = "already present"
		}
		b.add(domain.PhaseShell, domain.CommandStep{Label: "clone " + path.Base(c.dest), Argv: []string{"git", "clone", "--depth", "1", c.url, c.dest}}, skip)
	}
	shell, err := i.Probe.LoginShell(ctx)
	if err != nil {
		return fmt.Errorf("login shell: %w", err)
	}
	skip := ""
	if strings.HasSuffix(shell, "zsh") {
		skip = "shell is zsh"
	}
	b.add(domain.PhaseShell, domain.CommandStep{Label: "set login shell to zsh", Argv: []string{"chsh", "-s", zshBinary, i.User}, Sudo: true}, skip)
	return nil
}

func (i *Installer) planServices(b *planBuilder, d Draft) {
	var units []string
	for _, p := range i.Catalog {
		if d.includes(p) && p.Service != "" {
			units = append(units, p.Service)
		}
	}
	for _, dr := range i.selectedDrivers(d) {
		units = append(units, dr.Services...)
	}
	slices.Sort(units)
	units = slices.Compact(units)
	// Reading a missing link fails; that means no display manager is enabled.
	if current, _ := i.Service.Inspector.LinkTarget(displayManagerUnitLink); current != "" && path.Base(current) != lyUnit && slices.Contains(units, lyUnit) {
		units = slices.DeleteFunc(units, func(u string) bool { return u == lyUnit })
		b.plan.Notices = append(b.plan.Notices, fmt.Sprintf("%s is the enabled display manager: %s was not enabled", path.Base(current), lyUnit))
	}
	if len(units) > 0 {
		b.add(domain.PhaseServices, domain.CommandStep{Label: fmt.Sprintf("enable services (%d)", len(units)), Argv: slices.Concat([]string{"systemctl", "enable"}, units), Sudo: true}, "")
	}
}

func (i *Installer) planTheme(b *planBuilder, d Draft) {
	variant := d.ThemeVariant
	if variant == "" {
		variant = defaultVariant
	}
	dir := path.Join(i.Home, themeFilesDir)
	b.add(domain.PhaseTheme, domain.WriteFileStep{Path: path.Join(dir, "name"), Content: []byte(themeName + "\n"), Mode: installedFileMode}, "")
	b.add(domain.PhaseTheme, domain.WriteFileStep{Path: path.Join(dir, "variant"), Content: []byte(variant + "\n"), Mode: installedFileMode}, "")
}
