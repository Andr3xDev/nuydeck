package install

import (
	"slices"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	wantNuytekURL    = "https://github.com/Andr3xDev/nuytek-theme.git"
	wantNuytekCache  = testHome + "/.cache/nuydeck/nuytek-theme"
	wantThemesRoot   = testHome + "/.themes"
	wantObsidianDir  = wantThemesRoot + "/Abysal-Obsidian"
	wantMarbleDir    = wantThemesRoot + "/Abysal-Marble"
	wantSpicetifyGit = "https://github.com/spicetify/spicetify-themes"

	wantSpicetifyCache  = testHome + "/.cache/nuydeck/spicetify-themes"
	wantSpicetifyThemes = testHome + "/.config/spicetify/Themes"
	wantSpotifyDir      = testHome + "/.local/share/spotify-launcher/install/usr/share/spotify"
	wantSpotifyPrefs    = testHome + "/.config/spotify/prefs"

	skipLaunchedOnce  = "already launched once"
	skipDockerMember  = "already a docker group member"
	skipThemesPresent = "already present"
)

// newExtrasFixture is a fresh machine for the extras: no themes, no caches and no Spotify first run,
// with the Spotify packages in the full profile (the shared fixture reports unknown paths as existing).
func newExtrasFixture(t *testing.T) *planFixture {
	t.Helper()
	f := newPlanFixture(t)
	for _, p := range []string{
		wantNuytekCache, wantObsidianDir, wantMarbleDir, wantSpicetifyCache,
		wantSpicetifyThemes + "/Matte", wantSpotifyDir, wantSpotifyPrefs,
	} {
		f.in.kinds[p] = plan.KindNone
	}
	f.inst.Catalog = append(f.inst.Catalog,
		manifest.Package{Name: "spotify-launcher", Source: manifest.SourcePacman, Profiles: []string{"full"}},
		manifest.Package{Name: "spicetify-cli", Source: manifest.SourceAUR, Profiles: []string{"full"}},
	)
	return f
}

func findStep(p InstallPlan, argv ...string) (PlannedStep, int, bool) {
	for i, ps := range p.Steps {
		if c, ok := ps.Step.(plan.CommandStep); ok && slices.Equal(c.Argv, argv) {
			return ps, i, true
		}
	}
	return PlannedStep{}, -1, false
}

func mustFindStep(t *testing.T, p InstallPlan, argv ...string) (PlannedStep, int) {
	t.Helper()
	ps, i, ok := findStep(p, argv...)
	if !ok {
		t.Fatalf("no step with argv %v", argv)
	}
	return ps, i
}

func hasCommandNamed(p InstallPlan, names ...string) bool {
	for _, c := range stepsOfType[plan.CommandStep](rawSteps(p.Steps)) {
		if slices.Contains(names, c.Argv[0]) {
			return true
		}
	}
	return false
}

func TestPlan_DockerGroupStep(t *testing.T) {
	cases := []struct {
		name       string
		draft      Draft
		member     bool
		wantStep   bool
		wantSkip   string
		extraNames []string
	}{
		{name: "docker selected, not a member", draft: Draft{Profile: manifest.ProfileFull}, wantStep: true},
		{name: "already a member", draft: Draft{Profile: manifest.ProfileFull}, member: true, wantStep: true, wantSkip: skipDockerMember},
		{name: "docker not selected", draft: Draft{Profile: manifest.ProfileMinimal}},
		{name: "virtualization adds no libvirt group", draft: Draft{Profile: manifest.ProfileFull}, wantStep: true, extraNames: []string{"libvirt", "qemu-full", "virt-manager"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newExtrasFixture(t)
			f.draft = tc.draft
			f.probe.groups = map[string]bool{"docker": tc.member}
			for _, name := range tc.extraNames {
				f.inst.Catalog = append(f.inst.Catalog, manifest.Package{Name: name, Source: manifest.SourcePacman, Profiles: []string{"full"}})
			}
			installPlan := f.plan(t)

			var groupSteps []PlannedStep
			for _, ps := range installPlan.Steps {
				if c, ok := ps.Step.(plan.CommandStep); ok && slices.Contains([]string{"usermod", "gpasswd", "adduser"}, c.Argv[0]) {
					groupSteps = append(groupSteps, ps)
				}
			}
			if !tc.wantStep {
				if len(groupSteps) != 0 {
					t.Fatalf("group steps = %+v, want none", groupSteps)
				}
				return
			}
			if len(groupSteps) != 1 {
				t.Fatalf("group steps = %+v, want exactly the docker one", groupSteps)
			}
			got := groupSteps[0]
			c := got.Step.(plan.CommandStep)
			if !slices.Equal(c.Argv, []string{"usermod", "-aG", "docker", "u"}) || !c.Sudo || c.Interactive {
				t.Errorf("group step = %+v, want unattended sudo usermod -aG docker u", c)
			}
			if got.Phase != PhaseSystem || got.Skip != tc.wantSkip {
				t.Errorf("group step phase %d skip %q, want phase %d skip %q", got.Phase, got.Skip, PhaseSystem, tc.wantSkip)
			}
		})
	}
}

func TestPlan_FontPackageAddsNoDownloadOrExtractionStep(t *testing.T) {
	f := newExtrasFixture(t)
	f.inst.Catalog = append(f.inst.Catalog, manifest.Package{Name: "ttf-ms-fonts", Source: manifest.SourceAUR, Profiles: []string{"full"}})

	if hasCommandNamed(f.plan(t), "curl", "wget", "unzip", "tar", "bsdtar", "7z", "7za") {
		t.Error("plan holds a download or archive-extraction command, want the font to come from the package only")
	}
}

func TestPlan_GTKThemeIsClonedBuiltAndInstalledFromTheCacheInEveryProfile(t *testing.T) {
	for _, profile := range []string{manifest.ProfileMinimal, manifest.ProfileFull} {
		t.Run(profile, func(t *testing.T) {
			f := newExtrasFixture(t)
			f.draft = Draft{Profile: profile}
			installPlan := f.plan(t)

			clone, cloneAt := mustFindStep(t, installPlan, "git", "clone", "--depth", "1", wantNuytekURL, wantNuytekCache)
			build, buildAt := mustFindStep(t, installPlan, "bash", "build.sh")
			install, installAt := mustFindStep(t, installPlan, "bash", "install.sh", "-d", wantThemesRoot, "-l")

			if !(cloneAt < buildAt && buildAt < installAt) {
				t.Errorf("step order clone=%d build=%d install=%d, want clone, build, install", cloneAt, buildAt, installAt)
			}
			for name, ps := range map[string]PlannedStep{"clone": clone, "build": build, "install": install} {
				c := ps.Step.(plan.CommandStep)
				if ps.Phase != PhaseExtras || ps.Skip != "" || c.Sudo || c.UsesSudo || c.Interactive {
					t.Errorf("%s = %+v, want an unattended unprivileged runnable step in the extras phase", name, ps)
				}
			}
			for name, ps := range map[string]PlannedStep{"build": build, "install": install} {
				if dir := ps.Step.(plan.CommandStep).Dir; dir != wantNuytekCache+"/themes" {
					t.Errorf("%s Dir = %q, want the themes directory of the cache", name, dir)
				}
			}
		})
	}
}

func TestPlan_GTKThemeSkipsWhatIsAlreadyPresentAndRerunsPartialState(t *testing.T) {
	cases := []struct {
		name         string
		present      []string
		wantRun      [3]bool
		wantSkipText bool
	}{
		{name: "both themes present", present: []string{wantObsidianDir, wantMarbleDir}, wantRun: [3]bool{false, false, false}, wantSkipText: true},
		{name: "cache only", present: []string{wantNuytekCache}, wantRun: [3]bool{false, true, true}},
		{name: "one theme only", present: []string{wantMarbleDir}, wantRun: [3]bool{true, true, true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newExtrasFixture(t)
			for _, p := range tc.present {
				f.in.kinds[p] = plan.KindDir
			}
			installPlan := f.plan(t)

			clone, _ := mustFindStep(t, installPlan, "git", "clone", "--depth", "1", wantNuytekURL, wantNuytekCache)
			build, _ := mustFindStep(t, installPlan, "bash", "build.sh")
			install, _ := mustFindStep(t, installPlan, "bash", "install.sh", "-d", wantThemesRoot, "-l")
			for i, ps := range []PlannedStep{clone, build, install} {
				if runs := ps.Skip == ""; runs != tc.wantRun[i] {
					t.Errorf("step %d skip %q, want runs=%v", i, ps.Skip, tc.wantRun[i])
				}
			}
			if tc.wantSkipText && (build.Skip != skipThemesPresent || install.Skip != skipThemesPresent) {
				t.Errorf("build skip %q install skip %q, want %q", build.Skip, install.Skip, skipThemesPresent)
			}
		})
	}
}

func TestPlan_GTKThemeNoticeNamesGTK4DirAndTheStaleCacheOnlyWhenTheStepRuns(t *testing.T) {
	f := newExtrasFixture(t)
	notices := strings.ToLower(strings.Join(f.plan(t).Notices, "\n"))
	if !strings.Contains(notices, "gtk-4.0") || !strings.Contains(notices, "cache") {
		t.Errorf("notices = %q, want one naming ~/.config/gtk-4.0 and the stale cache", notices)
	}

	for _, p := range []string{wantObsidianDir, wantMarbleDir} {
		f.in.kinds[p] = plan.KindDir
	}
	if got := strings.Join(f.plan(t).Notices, "\n"); strings.Contains(got, "gtk-4.0") {
		t.Errorf("notices = %q, want no GTK notice when both themes exist", got)
	}
}

func TestPlan_SpotifyFirstRunLaunchesThenConfirmsThenGatesTheApplySteps(t *testing.T) {
	f := newExtrasFixture(t)
	installPlan := f.plan(t)

	launch, launchAt := mustFindStep(t, installPlan, "spotify-launcher")
	if c := launch.Step.(plan.CommandStep); !c.Interactive || c.Sudo || launch.Phase != PhaseExtras || launch.Skip != "" {
		t.Errorf("launch = %+v, want an interactive unprivileged runnable step in the extras phase", launch)
	}

	confirms := stepsOfType[plan.ConfirmStep](rawSteps(installPlan.Steps))
	if len(confirms) != 1 || confirms[0].ID == "" || !strings.Contains(confirms[0].Prompt, "Spotify") {
		t.Fatalf("confirm steps = %+v, want one with an ID and a Spotify prompt", confirms)
	}
	confirmAt := slices.IndexFunc(installPlan.Steps, func(ps PlannedStep) bool { _, ok := ps.Step.(plan.ConfirmStep); return ok })
	if confirmAt < launchAt {
		t.Errorf("confirm at %d before launch at %d", confirmAt, launchAt)
	}

	applyTheme := []string{"spicetify", "config", "current_theme", "Matte", "color_scheme", "Matte",
		"inject_css", "1", "replace_colors", "1", "overwrite_assets", "1", "inject_theme_js", "1"}
	for _, argv := range [][]string{applyTheme, {"spicetify", "backup", "apply"}} {
		ps, at := mustFindStep(t, installPlan, argv...)
		if ps.After != confirms[0].ID || at < confirmAt || ps.Phase != PhaseExtras {
			t.Errorf("%v: After %q at %d phase %d, want gated by %q after the confirm", argv, ps.After, at, ps.Phase, confirms[0].ID)
		}
	}
	for _, argv := range [][]string{
		{"spicetify", "config", "spotify_path", wantSpotifyDir},
		{"spicetify", "config", "prefs_path", wantSpotifyPrefs},
	} {
		if ps, _ := mustFindStep(t, installPlan, argv...); ps.After != "" || ps.Skip != "" {
			t.Errorf("%v = %+v, want an ungated runnable step", argv, ps)
		}
	}
}

func TestPlan_GatedSpicetifyStepsAreOptionalSoAFailureOnlyWarns(t *testing.T) {
	installPlan := newExtrasFixture(t).plan(t)

	applyTheme := []string{"spicetify", "config", "current_theme", "Matte", "color_scheme", "Matte",
		"inject_css", "1", "replace_colors", "1", "overwrite_assets", "1", "inject_theme_js", "1"}
	for _, argv := range [][]string{applyTheme, {"spicetify", "backup", "apply"}} {
		if ps, _ := mustFindStep(t, installPlan, argv...); !ps.Optional {
			t.Errorf("%v: Optional = false, want a failure to warn and the run to continue", argv)
		}
	}
}

func TestPlan_SpotifyConfirmDeclineNoticeTellsTheOwnerToRerunLater(t *testing.T) {
	confirms := stepsOfType[plan.ConfirmStep](rawSteps(newExtrasFixture(t).plan(t).Steps))
	if len(confirms) != 1 {
		t.Fatalf("confirm steps = %+v, want 1", confirms)
	}

	got := strings.ToLower(confirms[0].DeclineNotice)

	if !strings.Contains(got, "spicetify") || !strings.Contains(got, "re-run") {
		t.Errorf("DeclineNotice = %q, want it to mention spicetify and re-running the install later", confirms[0].DeclineNotice)
	}
}

func TestPlan_SpicetifyColorSchemeFollowsTheThemeVariant(t *testing.T) {
	cases := map[string]string{"": "Matte", "nadir": "Gray-Dark3", "zenit": "Porcelain"}
	for variant, scheme := range cases {
		t.Run("variant "+variant, func(t *testing.T) {
			f := newExtrasFixture(t)
			f.draft.ThemeVariant = variant

			mustFindStep(t, f.plan(t), "spicetify", "config", "current_theme", "Matte", "color_scheme", scheme,
				"inject_css", "1", "replace_colors", "1", "overwrite_assets", "1", "inject_theme_js", "1")
		})
	}
}

func TestPlan_SpicetifyThemesAreClonedToTheCacheAndOnlyMatteIsCopied(t *testing.T) {
	f := newExtrasFixture(t)
	installPlan := f.plan(t)

	clone := slices.IndexFunc(installPlan.Steps, func(ps PlannedStep) bool {
		c, ok := ps.Step.(plan.CommandStep)
		return ok && len(c.Argv) > 2 && c.Argv[0] == "git" && c.Argv[1] == "clone" &&
			c.Argv[len(c.Argv)-1] == wantSpicetifyCache && strings.HasPrefix(c.Argv[len(c.Argv)-2], wantSpicetifyGit)
	})
	if clone < 0 || installPlan.Steps[clone].Skip != "" {
		t.Fatalf("no runnable clone of %s into %s", wantSpicetifyGit, wantSpicetifyCache)
	}

	var copies []plan.CommandStep
	for _, c := range stepsOfType[plan.CommandStep](rawSteps(installPlan.Steps)) {
		if slices.ContainsFunc(c.Argv, func(a string) bool { return strings.HasPrefix(a, wantSpicetifyCache+"/") }) {
			copies = append(copies, c)
		}
	}
	if len(copies) != 1 || !slices.Contains(copies[0].Argv, wantSpicetifyCache+"/Matte") ||
		!slices.ContainsFunc(copies[0].Argv, func(a string) bool { return strings.HasPrefix(a, wantSpicetifyThemes) }) {
		t.Errorf("steps reading the cloned themes = %+v, want one copying only Matte into %s", copies, wantSpicetifyThemes)
	}
}

func TestPlan_SpicetifyCloneAndCopySkipWhenPresent(t *testing.T) {
	f := newExtrasFixture(t)
	f.in.kinds[wantSpicetifyCache] = plan.KindDir
	f.in.kinds[wantSpicetifyThemes+"/Matte"] = plan.KindDir

	for _, ps := range f.plan(t).Steps {
		c, ok := ps.Step.(plan.CommandStep)
		if !ok || ps.Skip != "" {
			continue
		}
		if slices.ContainsFunc(c.Argv, func(a string) bool { return strings.HasPrefix(a, wantSpicetifyCache) }) {
			t.Errorf("runnable step %v touches the spicetify theme cache, want clone and copy skipped", c.Argv)
		}
	}
}

func TestPlan_SpotifyLaunchAndConfirmAreSkippedOnlyWhenBothFirstRunFilesExist(t *testing.T) {
	cases := []struct {
		name    string
		present []string
		skipped bool
	}{
		{name: "both files exist", present: []string{wantSpotifyDir, wantSpotifyPrefs}, skipped: true},
		{name: "prefs missing", present: []string{wantSpotifyDir}},
		{name: "install dir missing", present: []string{wantSpotifyPrefs}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newExtrasFixture(t)
			for _, p := range tc.present {
				f.in.kinds[p] = plan.KindDir
			}
			installPlan := f.plan(t)

			launch, _ := mustFindStep(t, installPlan, "spotify-launcher")
			confirm := installPlan.Steps[slices.IndexFunc(installPlan.Steps, func(ps PlannedStep) bool { _, ok := ps.Step.(plan.ConfirmStep); return ok })]
			want := ""
			if tc.skipped {
				want = skipLaunchedOnce
			}
			if launch.Skip != want || confirm.Skip != want {
				t.Errorf("launch skip %q confirm skip %q, want %q for both", launch.Skip, confirm.Skip, want)
			}
		})
	}
}

func TestPlan_NoSpicetifyStepUnlessBothSpotifyPackagesAreSelected(t *testing.T) {
	cases := map[string][]string{
		"launcher only":  {"spotify-launcher"},
		"spicetify only": {"spicetify-cli"},
		"neither":        {"hyprland"},
	}
	for name, packages := range cases {
		t.Run(name, func(t *testing.T) {
			f := newExtrasFixture(t)
			f.draft = Draft{Profile: ProfileCustom, Packages: packages}
			installPlan := f.plan(t)

			if hasCommandNamed(installPlan, "spicetify", "spotify-launcher") ||
				len(stepsOfType[plan.ConfirmStep](rawSteps(installPlan.Steps))) != 0 {
				t.Errorf("plan holds spicetify, launch or confirm steps for %v", packages)
			}
		})
	}
}

func TestRun_DryRunListsTheExtrasThroughTheRunnerWithoutAsking(t *testing.T) {
	f := newExtrasFixture(t)
	f.probe.groups = map[string]bool{}
	installPlan := f.plan(t)
	r := &scriptedRunner{}
	inst := newRunInstaller(r)
	inst.DryRun = true

	res, events := runInstall(t, inst, installPlan.Steps, nil)

	if res.Err != nil {
		t.Fatalf("Err = %v", res.Err)
	}
	if slices.ContainsFunc(events, func(e plan.Event) bool { return e.Kind == plan.EventAwaitAck }) {
		t.Errorf("dry-run asked a question: %v", describe(events))
	}
	if len(stepsOfType[plan.ConfirmStep](r.ran())) != 1 {
		t.Error("Runner did not receive the Spotify confirm")
	}
	ranArgv := func(argv ...string) bool {
		return slices.ContainsFunc(stepsOfType[plan.CommandStep](r.ran()), func(c plan.CommandStep) bool { return slices.Equal(c.Argv, argv) })
	}
	for _, argv := range [][]string{
		{"usermod", "-aG", "docker", "u"},
		{"git", "clone", "--depth", "1", wantNuytekURL, wantNuytekCache},
		{"spotify-launcher"},
		{"spicetify", "backup", "apply"},
	} {
		if !ranArgv(argv...) {
			t.Errorf("dry-run did not list %v through the Runner", argv)
		}
	}
}
