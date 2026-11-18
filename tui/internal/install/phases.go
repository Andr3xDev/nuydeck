package install

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	pacmanConfFile         = "/etc/pacman.conf"
	logindDropInFile       = "/etc/systemd/logind.conf.d/60-nuydeck-power-key.conf"
	displayManagerUnitLink = "/etc/systemd/system/display-manager.service"
	lyUnit                 = "ly@tty1.service"
	lyTemplateUnit         = "ly@.service"

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

func (i *Installer) planMultilib(b *planBuilder) {
	content, ok := i.readSystemFile(b, pacmanConfFile)
	if !ok {
		return
	}
	enabled, changed, err := EnableMultilib(content)
	if err != nil {
		b.plan.Blocking = append(b.plan.Blocking, err)
		return
	}
	step := plan.WriteFileStep{Path: pacmanConfFile, Content: enabled, Mode: installedFileMode, Privileged: true, BackupTo: pacmanConfFile + i.backupSuffix()}
	skip := ""
	if !changed {
		skip = "multilib already active"
	}
	b.add(PhasePacmanConf, step, skip)
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
		if kind != plan.KindNone {
			cloneSkip = "clone already present"
		}
	}
	b.add(PhaseParuBootstrap, plan.CommandStep{Label: "clone paru", Argv: []string{"git", "clone", paruRepoURL, cacheDir}}, cloneSkip)
	b.add(PhaseParuBootstrap, plan.CommandStep{Label: "build and install paru", Argv: []string{"makepkg", "-si", "--noconfirm"}, Dir: cacheDir, UsesSudo: true}, paruSkip)
	b.add(PhaseAur, plan.CommandStep{Label: fmt.Sprintf("install AUR packages (%d)", len(aur)), Argv: slices.Concat(aurInstallArgs, aur), UsesSudo: true}, skipIfInstalled(aur, installed))
	return nil
}

func (i *Installer) planLogind(b *planBuilder) {
	existing, ok := i.readSystemFile(b, logindDropInFile)
	if !ok {
		return
	}
	step := plan.WriteFileStep{Path: logindDropInFile, Content: []byte(logindDropIn), Mode: installedFileMode, Privileged: true}
	skip := ""
	switch {
	case string(existing) == logindDropIn:
		skip = "drop-in already in place"
	case len(existing) > 0:
		step.BackupTo = logindDropInFile + i.backupSuffix()
	}
	b.add(PhaseSystem, step, skip)
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
		phase   Phase
		entries []manifest.ResolvedEntry
	}{{PhaseConfigs, configs}, {PhaseFonts, fonts}} {
		steps, notices, blocking, err := i.pushSteps(ctx, group.entries)
		if err != nil {
			return err
		}
		for _, s := range steps {
			b.add(group.phase, s, "")
		}
		b.plan.Notices = append(b.plan.Notices, notices...)
		b.plan.Blocking = append(b.plan.Blocking, blocking...)
		if group.phase == PhaseFonts {
			skip := ""
			if len(steps) == 0 {
				skip = "no fonts to copy"
			}
			b.add(PhaseFonts, plan.CommandStep{Label: "refresh font cache", Argv: []string{"fc-cache", "-f"}}, skip)
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
	step := plan.WriteSecretsStep{Path: secrets}
	for _, e := range d.Secrets {
		if e.Value != "" {
			step.Entries = append(step.Entries, e)
		}
	}
	if len(step.Entries) == 0 {
		return nil
	}
	kind, err := i.Service.Inspector.PathKind(secrets)
	if err != nil {
		return err
	}
	skip := ""
	if kind != plan.KindNone {
		skip = "left as is"
	}
	b.add(PhaseEnv, step, skip)
	return nil
}

func (i *Installer) planGPUEnvLink(b *planBuilder, d Draft) error {
	var ids []string
	for _, dr := range i.selectedDrivers(d) {
		ids = append(ids, dr.ID)
	}
	target := GPUEnvFileName(ids)
	if target == "" {
		return nil
	}
	link := plan.SymlinkStep{Path: path.Join(i.Home, homeGPUEnvDir, gpuEnvLinkName), Target: target}
	if d.SkipGPUEnvLink {
		b.add(PhaseEnv, link, "GPU env link skipped by owner")
		return nil
	}
	in := i.Service.Inspector
	repoKind, err := in.PathKind(path.Join(i.Service.RepoDir, repoGPUEnvDir, target))
	if err != nil {
		return err
	}
	if repoKind == plan.KindNone {
		b.plan.Notices = append(b.plan.Notices, fmt.Sprintf("%s not linked: %s is missing from the repo", gpuEnvLinkName, target))
		return nil
	}
	kind, err := in.PathKind(link.Path)
	if err != nil {
		return err
	}
	switch kind {
	case plan.KindNone:
		b.add(PhaseEnv, link, "")
	case plan.KindLink:
		current, _ := in.LinkTarget(link.Path)
		skip := ""
		if current == target {
			skip = "already linked"
		}
		b.add(PhaseEnv, link, skip)
	default:
		b.plan.Notices = append(b.plan.Notices, fmt.Sprintf("%s is a regular file and was left as is", link.Path))
		b.add(PhaseEnv, link, "regular file kept")
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
		if kind != plan.KindNone {
			skip = "already present"
		}
		b.add(PhaseShell, plan.CommandStep{Label: "clone " + path.Base(c.dest), Argv: []string{"git", "clone", "--depth", "1", c.url, c.dest}}, skip)
	}
	shell, err := i.Probe.LoginShell(ctx)
	if err != nil {
		return fmt.Errorf("login shell: %w", err)
	}
	skip := ""
	if strings.HasSuffix(shell, "zsh") {
		skip = "shell is zsh"
	}
	b.add(PhaseShell, plan.CommandStep{Label: "set login shell to zsh", Argv: []string{"chsh", "-s", zshBinary, i.User}, Sudo: true}, skip)
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
	if current, _ := i.Service.Inspector.LinkTarget(displayManagerUnitLink); current != "" && path.Base(current) != lyTemplateUnit && slices.Contains(units, lyUnit) {
		units = slices.DeleteFunc(units, func(u string) bool { return u == lyUnit })
		b.plan.Notices = append(b.plan.Notices, fmt.Sprintf("%s is the enabled display manager: %s was not enabled", path.Base(current), lyUnit))
	}
	if len(units) > 0 {
		b.add(PhaseServices, plan.CommandStep{Label: fmt.Sprintf("enable services (%d)", len(units)), Argv: slices.Concat([]string{"systemctl", "enable"}, units), Sudo: true}, "")
	}
}

func (i *Installer) planTheme(b *planBuilder, d Draft) {
	variant := d.ThemeVariant
	if variant == "" {
		variant = defaultVariant
	}
	dir := path.Join(i.Home, themeFilesDir)
	b.add(PhaseTheme, plan.WriteFileStep{Path: path.Join(dir, "name"), Content: []byte(themeName + "\n"), Mode: installedFileMode}, "")
	b.add(PhaseTheme, plan.WriteFileStep{Path: path.Join(dir, "variant"), Content: []byte(variant + "\n"), Mode: installedFileMode}, "")
}
