package install

import (
	"context"
	"fmt"
	"path"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	nuytekThemeURL = "https://github.com/Andr3xDev/nuytek-theme.git"
	nuytekCacheAt  = ".cache/nuydeck/nuytek-theme"
	themesRootAt   = ".themes"

	spicetifyThemesURL   = "https://github.com/spicetify/spicetify-themes"
	spicetifyCacheAt     = ".cache/nuydeck/spicetify-themes"
	spicetifyThemesDirAt = ".config/spicetify/Themes"
	spotifyInstallAt     = ".local/share/spotify-launcher/install/usr/share/spotify"
	spotifyPrefsAt       = ".config/spotify/prefs"
	spicetifyTheme       = "Matte"
	spotifyOpenedID      = "spotify-opened"
	spotifyLaunchID      = "spotify-launch"

	skipPresent      = "already present"
	skipFirstRunDone = "already launched once"
)

var gtkThemeNames = []string{"Abysal-Obsidian", "Abysal-Marble"}

func (i *Installer) planDockerGroup(ctx context.Context, b *planBuilder, d Draft) error {
	if !i.selected(d, "docker") {
		return nil
	}
	member, err := i.Probe.InGroup(ctx, "docker")
	if err != nil {
		return fmt.Errorf("docker group: %w", err)
	}
	skip := ""
	if member {
		skip = "already a docker group member"
	}
	b.add(PhaseSystem, plan.CommandStep{Label: "add " + i.User + " to the docker group", Argv: []string{"usermod", "-aG", "docker", i.User}, Sudo: true}, skip)
	return nil
}

func (i *Installer) planExtras(b *planBuilder, d Draft) error {
	if err := i.planGTKTheme(b); err != nil {
		return err
	}
	if i.selected(d, "spotify-launcher") && i.selected(d, "spicetify-cli") {
		return i.planSpicetify(b, d)
	}
	return nil
}

func (i *Installer) exists(p string) (bool, error) {
	kind, err := i.Service.Inspector.PathKind(p)
	return kind != plan.KindNone, err
}

func (i *Installer) planGTKTheme(b *planBuilder) error {
	cache := path.Join(i.Home, nuytekCacheAt)
	themesRoot := path.Join(i.Home, themesRootAt)
	cached, err := i.exists(cache)
	if err != nil {
		return err
	}
	allThemes := true
	for _, name := range gtkThemeNames {
		present, err := i.exists(path.Join(themesRoot, name))
		if err != nil {
			return err
		}
		allThemes = allThemes && present
	}

	buildSkip, cloneSkip := "", ""
	if allThemes {
		buildSkip, cloneSkip = skipPresent, skipPresent
	} else if cached {
		cloneSkip = skipPresent
	}
	scripts := path.Join(cache, "themes")
	b.add(PhaseExtras, plan.CommandStep{Label: "clone nuytek-theme", Argv: []string{"git", "clone", "--depth", "1", nuytekThemeURL, cache}}, cloneSkip)
	b.add(PhaseExtras, plan.CommandStep{Label: "build GTK theme", Argv: []string{"bash", "build.sh"}, Dir: scripts}, buildSkip)
	b.add(PhaseExtras, plan.CommandStep{Label: "install GTK theme", Argv: []string{"bash", "install.sh", "-d", themesRoot, "-l"}, Dir: scripts}, buildSkip)
	if !allThemes {
		b.plan.Notices = append(b.plan.Notices, "GTK theme install links ~/.config/gtk-4.0; an existing nuytek-theme cache is reused and not updated")
	}
	return nil
}

func (i *Installer) planSpicetify(b *planBuilder, d Draft) error {
	cache := path.Join(i.Home, spicetifyCacheAt)
	themesDir := path.Join(i.Home, spicetifyThemesDirAt)
	spotifyDir := path.Join(i.Home, spotifyInstallAt)
	prefs := path.Join(i.Home, spotifyPrefsAt)

	cached, err := i.exists(cache)
	if err != nil {
		return err
	}
	copied, err := i.exists(path.Join(themesDir, spicetifyTheme))
	if err != nil {
		return err
	}
	hasInstall, err := i.exists(spotifyDir)
	if err != nil {
		return err
	}
	hasPrefs, err := i.exists(prefs)
	if err != nil {
		return err
	}
	skipIf := func(done bool, reason string) string {
		if done {
			return reason
		}
		return ""
	}
	cloneSkip := skipIf(cached, skipPresent)
	copySkip := skipIf(copied, skipPresent)
	launchSkip := skipIf(hasInstall && hasPrefs, skipFirstRunDone)

	scheme := "Matte"
	switch d.ThemeVariant {
	case "zenit":
		scheme = "Porcelain"
	case "nadir":
		scheme = "Gray-Dark3"
	}
	b.add(PhaseExtras, plan.CommandStep{Label: "spicetify spotify_path", Argv: []string{"spicetify", "config", "spotify_path", spotifyDir}}, "")
	b.add(PhaseExtras, plan.CommandStep{Label: "spicetify prefs_path", Argv: []string{"spicetify", "config", "prefs_path", prefs}}, "")
	b.add(PhaseExtras, plan.CommandStep{Label: "clone spicetify-themes", Argv: []string{"git", "clone", "--depth", "1", spicetifyThemesURL, cache}}, cloneSkip)
	b.add(PhaseExtras, plan.CommandStep{Label: "create spicetify themes dir", Argv: []string{"mkdir", "-p", themesDir}}, copySkip)
	b.add(PhaseExtras, plan.CommandStep{Label: "copy spicetify theme " + spicetifyTheme, Argv: []string{"cp", "-r", path.Join(cache, spicetifyTheme), themesDir}}, copySkip)
	b.plan.Steps = append(b.plan.Steps,
		PlannedStep{Phase: PhaseExtras, Step: plan.CommandStep{Label: "launch spotify-launcher", Argv: []string{"spotify-launcher"}, Interactive: true}, ID: spotifyLaunchID, Skip: launchSkip, Optional: true},
		PlannedStep{Phase: PhaseExtras, Step: plan.ConfirmStep{ID: spotifyOpenedID, Prompt: "Did Spotify open? Close it, then answer.",
			DeclineNotice: "Spicetify theme not applied; re-run the install later once Spotify has opened once."}, Skip: launchSkip, After: spotifyLaunchID},
	)
	gate := func(step plan.CommandStep) {
		b.plan.Steps = append(b.plan.Steps, PlannedStep{Phase: PhaseExtras, Step: step, After: spotifyOpenedID, Optional: true})
	}
	gate(plan.CommandStep{Label: "spicetify apply theme", Argv: []string{"spicetify", "config", "current_theme", spicetifyTheme, "color_scheme", scheme,
		"inject_css", "1", "replace_colors", "1", "overwrite_assets", "1", "inject_theme_js", "1"}})
	gate(plan.CommandStep{Label: "spicetify backup apply", Argv: []string{"spicetify", "backup", "apply"}})
	return nil
}
