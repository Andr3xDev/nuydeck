package app

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	pacmanConfPath     = "/etc/pacman.conf"
	backupConfPath     = pacmanConfPath + ".nuydeck-bak-20261104-120000"
	logindDropInPath   = "/etc/systemd/logind.conf.d/60-nuydeck-power-key.conf"
	displayManagerLink = "/etc/systemd/system/display-manager.service"

	gpuEnvRepoDir  = testRepo + "/config/hypr/configs"
	gpuEnvLinkPath = testHome + "/.config/hypr/configs/gpu_env.lua"
	secretsPath    = testHome + "/.secrets"
	ohMyZshPath    = testHome + "/.oh-my-zsh"
	pluginsDir     = ohMyZshPath + "/custom/plugins"
	paruCacheDir   = testHome + "/.cache/nuydeck/paru-bootstrap"
	themeDir       = testHome + "/.config/nuydeck/theme"

	commentedPacmanConf  = "[options]\nHoldPkg = pacman glibc\n\n#[multilib]\n#Include = /etc/pacman.d/mirrorlist\n"
	activePacmanConf     = "[options]\nHoldPkg = pacman glibc\n\n[multilib]\nInclude = /etc/pacman.d/mirrorlist\n"
	unexpectedPacmanConf = "[options]\nHoldPkg = pacman glibc\n"
)

var gpuEnvFiles = []string{
	"gpu_env_amd.lua", "gpu_env_intel.lua", "gpu_env_nvidia.lua",
	"gpu_env_amd_intel.lua", "gpu_env_amd_nvidia.lua", "gpu_env_intel_nvidia.lua",
	"gpu_env_amd_intel_nvidia.lua",
}

type planFixture struct {
	probe  *fakeProbe
	in     *fakeInspector
	runner *fakeRunner
	inst   *Installer
	draft  Draft
}

// newPlanFixture is a fresh machine: nothing installed, multilib commented out, bash shell, all seven GPU env files in the repo copy.
func newPlanFixture(t *testing.T) *planFixture {
	t.Helper()
	kinds := map[string]plan.NodeKind{
		secretsPath:                             plan.KindNone,
		ohMyZshPath:                             plan.KindNone,
		pluginsDir + "/zsh-syntax-highlighting": plan.KindNone,
		pluginsDir + "/zsh-autosuggestions":     plan.KindNone,
		paruCacheDir:                            plan.KindNone,
		gpuEnvLinkPath:                          plan.KindNone,
	}
	for _, name := range gpuEnvFiles {
		kinds[gpuEnvRepoDir+"/"+name] = plan.KindFile
	}
	in := &fakeInspector{
		kinds:       kinds,
		linkTargets: map[string]string{},
		diff: func(e manifest.ResolvedEntry, _, _ plan.Endpoint) ([]plan.Change, error) {
			if e.Root == manifest.RootFonts {
				return []plan.Change{change("a.ttf", plan.Create, plan.KindFile)}, nil
			}
			return []plan.Change{
				change("a.conf", plan.Create, plan.KindFile),
				change("b.conf", plan.Update, plan.KindFile),
				change("old.conf", plan.Delete, plan.KindFile),
			}, nil
		},
	}
	runner := &fakeRunner{}
	probe := &fakeProbe{
		installed: map[string]bool{},
		commands:  map[string]bool{},
		shell:     "/bin/bash",
		files:     map[string][]byte{pacmanConfPath: []byte(commentedPacmanConf)},
	}
	inst := &Installer{
		Service: newService(t, in, runner),
		Probe:   probe,
		Runner:  runner,
		Preview: runner,
		Now:     func() time.Time { return testNow },
		Catalog: []manifest.Package{
			{Name: "hyprland", Source: manifest.SourcePacman, Profiles: []string{"minimal", "full"}},
			{Name: "networkmanager", Source: manifest.SourcePacman, Profiles: []string{"minimal", "full"}, Service: "NetworkManager.service"},
			{Name: "ly", Source: manifest.SourcePacman, Profiles: []string{"minimal", "full"}, Service: "ly.service"},
			{Name: "docker", Source: manifest.SourcePacman, Profiles: []string{"full"}, Service: "docker.service"},
			{Name: "steam", Source: manifest.SourcePacman, Profiles: []string{"full"}},
			{Name: "zen-browser-bin", Source: manifest.SourceAUR, Profiles: []string{"full"}},
		},
		Drivers: []manifest.Driver{
			{ID: "intel", VendorID: "8086", Packages: []string{"intel-media-driver", "vulkan-intel"}},
			{ID: "nvidia", VendorID: "10de", Packages: []string{"nvidia-open-dkms", "nvidia-utils"},
				Services: []string{"nvidia-suspend.service", "nvidia-hibernate.service", "nvidia-resume.service"}},
			{ID: "amd", VendorID: "1002", Packages: []string{"mesa", "vulkan-radeon"}},
		},
		Entries: []manifest.ResolvedEntry{
			resolved(manifest.RootConfig, "hypr"),
			{
				Entry:    manifest.Entry{Root: manifest.RootFonts, Path: "JetBrains"},
				RepoPath: testRepo + "/fonts/JetBrains",
				DestPath: testHome + "/.local/share/fonts/JetBrains",
			},
		},
		Home: testHome,
		User: "u",
	}
	return &planFixture{
		probe: probe, in: in, runner: runner, inst: inst,
		draft: Draft{Profile: manifest.ProfileFull, Drivers: []string{"intel", "nvidia"}},
	}
}

func (f *planFixture) plan(t *testing.T) InstallPlan {
	t.Helper()
	installPlan, err := f.inst.Plan(context.Background(), f.draft)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return installPlan
}

func phaseSteps(installPlan InstallPlan, phase domain.Phase) []domain.PlannedStep {
	var out []domain.PlannedStep
	for _, ps := range installPlan.Steps {
		if ps.Phase == phase {
			out = append(out, ps)
		}
	}
	return out
}

func runnable(steps []domain.PlannedStep) []domain.PlannedStep {
	var out []domain.PlannedStep
	for _, ps := range steps {
		if ps.Skip == "" {
			out = append(out, ps)
		}
	}
	return out
}

func rawSteps(steps []domain.PlannedStep) []plan.Step {
	out := make([]plan.Step, len(steps))
	for i, ps := range steps {
		out[i] = ps.Step
	}
	return out
}

func runnableCommands(installPlan InstallPlan, phase domain.Phase) []plan.CommandStep {
	return stepsOfType[plan.CommandStep](rawSteps(runnable(phaseSteps(installPlan, phase))))
}

func runnableWrites(installPlan InstallPlan, path string) []plan.WriteFileStep {
	var out []plan.WriteFileStep
	for _, w := range stepsOfType[plan.WriteFileStep](rawSteps(runnable(installPlan.Steps))) {
		if w.Path == path {
			out = append(out, w)
		}
	}
	return out
}

func cloneDestinations(commands []plan.CommandStep) []string {
	var out []string
	for _, c := range commands {
		if len(c.Argv) > 1 && c.Argv[0] == "git" && c.Argv[1] == "clone" {
			out = append(out, c.Argv[len(c.Argv)-1])
		}
	}
	return sorted(out...)
}

func TestPlan_PhasesFollowFixedOrderWhateverTheManifestOrder(t *testing.T) {
	f := newPlanFixture(t)
	installPlan := f.plan(t)

	var phases []domain.Phase
	for _, ps := range installPlan.Steps {
		phases = append(phases, ps.Phase)
	}
	if !slices.IsSorted(phases) {
		t.Errorf("phases are not in execution order: %v", phases)
	}
	for phase := domain.PhasePrivilege; phase <= domain.PhaseReboot; phase++ {
		if len(phaseSteps(installPlan, phase)) == 0 {
			t.Errorf("phase %d has no step in a full install on a fresh machine", phase)
		}
	}

	reversed := newPlanFixture(t)
	slices.Reverse(reversed.inst.Catalog)
	slices.Reverse(reversed.inst.Drivers)
	slices.Reverse(reversed.draft.Drivers)
	var reversedPhases []domain.Phase
	for _, ps := range reversed.plan(t).Steps {
		reversedPhases = append(reversedPhases, ps.Phase)
	}
	if !slices.Equal(phases, reversedPhases) {
		t.Errorf("phase sequence changed with manifest order: %v vs %v", phases, reversedPhases)
	}

	if len(installPlan.Notices) != 0 || len(installPlan.Blocking) != 0 {
		t.Errorf("fresh machine plan has notices %v / blocking %v", installPlan.Notices, installPlan.Blocking)
	}
	if got := f.runner.steps; len(got) != 0 {
		t.Errorf("Plan ran steps before confirmation: %v", got)
	}
	logs := stepsOfType[plan.AppendLogStep](rawSteps(phaseSteps(installPlan, domain.PhaseRecord)))
	if len(logs) != 1 || logs[0].Entry.Op != "install" {
		t.Errorf("record phase = %+v, want one AppendLogStep with op install", logs)
	}
}

func TestPlan_OnlySudoHandoffIsInteractiveAndRebootIsLast(t *testing.T) {
	installPlan := newPlanFixture(t).plan(t)

	var interactive []plan.CommandStep
	for _, c := range stepsOfType[plan.CommandStep](rawSteps(installPlan.Steps)) {
		if c.Interactive {
			interactive = append(interactive, c)
		}
	}
	if len(interactive) != 1 || !slices.Equal(interactive[0].Argv, []string{"sudo", "-v"}) {
		t.Fatalf("interactive steps = %+v, want exactly sudo -v", interactive)
	}
	if first, ok := installPlan.Steps[0].Step.(plan.CommandStep); !ok || !first.Interactive {
		t.Errorf("first step = %+v, want the sudo -v handoff", installPlan.Steps[0])
	}

	last := installPlan.Steps[len(installPlan.Steps)-1]
	reboot, ok := last.Step.(plan.CommandStep)
	if !ok || last.Phase != domain.PhaseReboot || last.Skip != "" ||
		!slices.Equal(reboot.Argv, []string{"systemctl", "reboot"}) || !reboot.Sudo {
		t.Fatalf("last step = %+v, want a never-skipped sudo systemctl reboot", last)
	}
}

func TestPlan_NeverPlansPackageRemoval(t *testing.T) {
	installPlan := newPlanFixture(t).plan(t)
	for _, c := range stepsOfType[plan.CommandStep](rawSteps(installPlan.Steps)) {
		if c.Argv[0] != "pacman" && c.Argv[0] != "paru" {
			continue
		}
		for _, arg := range c.Argv {
			if strings.HasPrefix(arg, "-R") || arg == "--remove" {
				t.Errorf("removal argv planned: %v", c.Argv)
			}
		}
	}
}

func TestPlan_PacmanStepUpgradesAndInstallsSelection(t *testing.T) {
	cases := []struct {
		name       string
		draft      Draft
		wantPacman []string
		wantAur    []string
	}{
		{
			name:       "minimal selects only minimal packages",
			draft:      Draft{Profile: manifest.ProfileMinimal},
			wantPacman: []string{"hyprland", "ly", "networkmanager"},
		},
		{
			name:  "full adds drivers and the paru prerequisites",
			draft: Draft{Profile: manifest.ProfileFull, Drivers: []string{"nvidia", "intel"}},
			wantPacman: []string{
				"base-devel", "docker", "git", "hyprland", "intel-media-driver", "ly", "networkmanager",
				"nvidia-open-dkms", "nvidia-utils", "steam", "vulkan-intel",
			},
			wantAur: []string{"zen-browser-bin"},
		},
		{
			name:       "custom installs only the marked packages",
			draft:      Draft{Profile: ProfileCustom, Packages: []string{"hyprland", "zen-browser-bin"}},
			wantPacman: []string{"base-devel", "git", "hyprland"},
			wantAur:    []string{"zen-browser-bin"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			f.draft = tc.draft
			installPlan := f.plan(t)

			pacman := runnableCommands(installPlan, domain.PhasePacman)
			if len(pacman) != 1 {
				t.Fatalf("runnable pacman steps = %d, want 1", len(pacman))
			}
			wantPrefix := []string{"pacman", "-Syu", "--needed", "--noconfirm"}
			if !pacman[0].Sudo || !slices.Equal(pacman[0].Argv[:len(wantPrefix)], wantPrefix) {
				t.Errorf("pacman step = %+v, want sudo pacman -Syu --needed --noconfirm", pacman[0])
			}
			if got := sorted(pacman[0].Argv[len(wantPrefix):]...); !slices.Equal(got, tc.wantPacman) {
				t.Errorf("pacman packages = %v, want %v", got, tc.wantPacman)
			}

			aur := runnableCommands(installPlan, domain.PhaseAur)
			if len(tc.wantAur) == 0 {
				if len(aur) != 0 {
					t.Errorf("AUR step planned without AUR packages: %+v", aur)
				}
				return
			}
			if len(aur) != 1 {
				t.Fatalf("runnable AUR steps = %d, want 1", len(aur))
			}
			wantAurPrefix := []string{"paru", "-S", "--needed", "--noconfirm", "--skipreview", "--sudoflags", "-n"}
			if aur[0].Sudo || !aur[0].UsesSudo || aur[0].Interactive || !slices.Equal(aur[0].Argv[:len(wantAurPrefix)], wantAurPrefix) {
				t.Errorf("AUR step = %+v, want unattended paru without sudo wrapper", aur[0])
			}
			if got := sorted(aur[0].Argv[len(wantAurPrefix):]...); !slices.Equal(got, tc.wantAur) {
				t.Errorf("AUR packages = %v, want %v", got, tc.wantAur)
			}
		})
	}
}

func TestPlan_ParuBootstrapOnlyWhenNeededAndUnattended(t *testing.T) {
	cases := []struct {
		name        string
		mutate      func(f *planFixture)
		wantClone   bool
		wantMakepkg bool
	}{
		{name: "paru absent", mutate: func(*planFixture) {}, wantClone: true, wantMakepkg: true},
		{name: "paru present", mutate: func(f *planFixture) { f.probe.commands["paru"] = true }},
		{name: "clone cache exists", mutate: func(f *planFixture) { f.in.kinds[paruCacheDir] = plan.KindDir }, wantMakepkg: true},
		{name: "no AUR package", mutate: func(f *planFixture) { f.draft.Profile = manifest.ProfileMinimal }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			tc.mutate(f)
			steps := runnableCommands(f.plan(t), domain.PhaseParuBootstrap)

			var clones, makepkgs []plan.CommandStep
			for _, c := range steps {
				switch c.Argv[0] {
				case "git":
					clones = append(clones, c)
				case "makepkg":
					makepkgs = append(makepkgs, c)
				}
			}
			if (len(clones) == 1) != tc.wantClone || len(clones) > 1 {
				t.Errorf("runnable clones = %+v, want present=%v", clones, tc.wantClone)
			}
			if (len(makepkgs) == 1) != tc.wantMakepkg || len(makepkgs) > 1 {
				t.Fatalf("runnable makepkg = %+v, want present=%v", makepkgs, tc.wantMakepkg)
			}
			for _, c := range clones {
				if got := c.Argv[len(c.Argv)-1]; got != paruCacheDir {
					t.Errorf("clone destination = %q, want %q", got, paruCacheDir)
				}
			}
			for _, c := range makepkgs {
				if !slices.Equal(c.Argv, []string{"makepkg", "-si", "--noconfirm"}) || c.Dir != paruCacheDir || c.Sudo || !c.UsesSudo || c.Interactive {
					t.Errorf("makepkg step = %+v, want unattended non-sudo makepkg -si --noconfirm in the clone dir", c)
				}
			}
		})
	}
}

func TestPlan_MultilibEditRunsOnlyForTheExpectedCommentedLines(t *testing.T) {
	enabled, _, err := domain.EnableMultilib([]byte(commentedPacmanConf))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		conf         string
		wantWrite    bool
		wantBlocking bool
	}{
		{name: "commented out", conf: commentedPacmanConf, wantWrite: true},
		{name: "already active", conf: activePacmanConf},
		{name: "unexpected file", conf: unexpectedPacmanConf, wantBlocking: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			f.probe.files[pacmanConfPath] = []byte(tc.conf)
			installPlan := f.plan(t)

			writes := runnableWrites(installPlan, pacmanConfPath)
			if (len(writes) == 1) != tc.wantWrite || len(writes) > 1 {
				t.Fatalf("runnable pacman.conf writes = %+v, want present=%v", writes, tc.wantWrite)
			}
			if (len(installPlan.Blocking) > 0) != tc.wantBlocking {
				t.Errorf("blocking = %v, want blocking=%v", installPlan.Blocking, tc.wantBlocking)
			}
			if !tc.wantWrite {
				return
			}
			w := writes[0]
			if !w.Privileged || w.BackupTo != backupConfPath || !bytes.Equal(w.Content, enabled) {
				t.Errorf("write = {Privileged:%v BackupTo:%q}, want privileged, backup %q and the multilib-enabled content", w.Privileged, w.BackupTo, backupConfPath)
			}
		})
	}
}

func TestPlan_LogindDropInIsPlannedAfterAurAndBackedUpWhenDifferent(t *testing.T) {
	identical := []byte("[Login]\nHandlePowerKey=ignore\nHandlePowerKeyLongPress=poweroff\n")
	cases := []struct {
		name       string
		existing   []byte
		wantWrite  bool
		wantBackup bool
	}{
		{name: "absent", wantWrite: true},
		{name: "identical", existing: identical},
		{name: "different", existing: []byte("[Login]\nHandlePowerKey=suspend\n"), wantWrite: true, wantBackup: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			if tc.existing != nil {
				f.probe.files[logindDropInPath] = tc.existing
			}
			installPlan := f.plan(t)

			writes := runnableWrites(installPlan, logindDropInPath)
			if (len(writes) == 1) != tc.wantWrite || len(writes) > 1 {
				t.Fatalf("runnable drop-in writes = %+v, want present=%v", writes, tc.wantWrite)
			}
			if !tc.wantWrite {
				return
			}
			w := writes[0]
			content := string(w.Content)
			if !w.Privileged || !strings.Contains(content, "HandlePowerKey=ignore") || !strings.Contains(content, "HandlePowerKeyLongPress=poweroff") {
				t.Errorf("drop-in = %q privileged=%v, want short press ignore and long press poweroff", content, w.Privileged)
			}
			if (w.BackupTo != "") != tc.wantBackup {
				t.Errorf("BackupTo = %q, want backup=%v", w.BackupTo, tc.wantBackup)
			}
			if got := phaseSteps(installPlan, domain.PhaseSystem); len(got) == 0 {
				t.Error("drop-in is not in the system phase")
			}
		})
	}
}

func TestPlan_ConfigsAndFontsReuseThePushPlanWithoutDeletes(t *testing.T) {
	installPlan := newPlanFixture(t).plan(t)

	configSteps := rawSteps(phaseSteps(installPlan, domain.PhaseConfigs))
	copies := stepsOfType[plan.CopyStep](configSteps)
	if len(copies) != 1 || !slices.Equal(copies[0].Paths, []string{"a.conf", "b.conf"}) || copies[0].Direction != plan.Push {
		t.Fatalf("config copies = %+v, want one push copy of a.conf and b.conf", copies)
	}
	backupAt, copyAt := -1, -1
	for i, s := range configSteps {
		switch s.(type) {
		case plan.BackupStep:
			backupAt = i
		case plan.CopyStep:
			copyAt = i
		}
	}
	if backupAt < 0 || backupAt > copyAt {
		t.Errorf("backup index %d, copy index %d: an overwrite needs a backup step first", backupAt, copyAt)
	}

	fontSteps := rawSteps(phaseSteps(installPlan, domain.PhaseFonts))
	if got := stepsOfType[plan.CopyStep](fontSteps); len(got) != 1 || !slices.Equal(got[0].Paths, []string{"a.ttf"}) {
		t.Errorf("font copies = %+v, want one copy of a.ttf", got)
	}
	if got := runnableCommands(installPlan, domain.PhaseFonts); len(got) != 1 || !slices.Equal(got[0].Argv, []string{"fc-cache", "-f"}) {
		t.Errorf("font phase commands = %+v, want fc-cache -f", got)
	}
	if got := stepsOfType[plan.DeleteStep](rawSteps(installPlan.Steps)); len(got) != 0 {
		t.Errorf("install planned deletions: %+v", got)
	}
}

func TestPlan_BlockedSyncItemIsABlockingError(t *testing.T) {
	f := newPlanFixture(t)
	f.in.diff = diffReturning(change("sub/a.conf", plan.Create, plan.KindFile))
	f.in.kinds[testHome+"/.config/hypr/sub"] = plan.KindLink

	if installPlan := f.plan(t); len(installPlan.Blocking) == 0 {
		t.Fatal("blocked push item produced no blocking error")
	}
}

func TestPlan_GPUEnvLinkTargetsTheFileOfTheSelectedDrivers(t *testing.T) {
	cases := []struct {
		drivers []string
		want    string
	}{
		{[]string{"amd"}, "gpu_env_amd.lua"},
		{[]string{"intel"}, "gpu_env_intel.lua"},
		{[]string{"nvidia"}, "gpu_env_nvidia.lua"},
		{[]string{"nvidia", "amd"}, "gpu_env_amd_nvidia.lua"},
		{[]string{"intel", "amd"}, "gpu_env_amd_intel.lua"},
		{[]string{"nvidia", "intel"}, "gpu_env_intel_nvidia.lua"},
		{[]string{"nvidia", "intel", "amd"}, "gpu_env_amd_intel_nvidia.lua"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			f := newPlanFixture(t)
			f.draft.Drivers = tc.drivers
			links := stepsOfType[plan.SymlinkStep](rawSteps(runnable(phaseSteps(f.plan(t), domain.PhaseEnv))))
			if len(links) != 1 || links[0].Path != gpuEnvLinkPath || links[0].Target != tc.want {
				t.Fatalf("links = %+v, want %s -> %s", links, gpuEnvLinkPath, tc.want)
			}
		})
	}
}

func TestPlan_GPUEnvLinkCases(t *testing.T) {
	const wantTarget = "gpu_env_intel_nvidia.lua"
	cases := []struct {
		name         string
		mutate       func(f *planFixture)
		wantSteps    int
		wantRunnable bool
		wantNotice   string
	}{
		{name: "no link yet", mutate: func(*planFixture) {}, wantSteps: 1, wantRunnable: true},
		{
			name: "same target already linked",
			mutate: func(f *planFixture) {
				f.in.kinds[gpuEnvLinkPath] = plan.KindLink
				f.in.linkTargets[gpuEnvLinkPath] = wantTarget
			},
			wantSteps: 1,
		},
		{
			name: "other target is replaced",
			mutate: func(f *planFixture) {
				f.in.kinds[gpuEnvLinkPath] = plan.KindLink
				f.in.linkTargets[gpuEnvLinkPath] = "gpu_env_nvidia.lua"
			},
			wantSteps: 1, wantRunnable: true,
		},
		{
			name:       "regular file is kept and warned about",
			mutate:     func(f *planFixture) { f.in.kinds[gpuEnvLinkPath] = plan.KindFile },
			wantSteps:  1,
			wantNotice: "gpu_env.lua",
		},
		{
			name:      "owner skips the link",
			mutate:    func(f *planFixture) { f.draft.SkipGPUEnvLink = true },
			wantSteps: 1,
		},
		{
			name:   "only unknown driver ids plan no link",
			mutate: func(f *planFixture) { f.draft.Drivers = []string{"ghost"} },
		},
		{
			name:   "owner skip without drivers plans no link",
			mutate: func(f *planFixture) { f.draft.Drivers, f.draft.SkipGPUEnvLink = nil, true },
		},
		{
			name:       "no file for the combination",
			mutate:     func(f *planFixture) { f.in.kinds[gpuEnvRepoDir+"/"+wantTarget] = plan.KindNone },
			wantNotice: wantTarget,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			tc.mutate(f)
			installPlan := f.plan(t)

			links := stepsOfType[plan.SymlinkStep](rawSteps(installPlan.Steps))
			if len(links) != tc.wantSteps {
				t.Fatalf("symlink steps = %d, want %d", len(links), tc.wantSteps)
			}
			if got := len(stepsOfType[plan.SymlinkStep](rawSteps(runnable(installPlan.Steps)))) == 1; got != tc.wantRunnable {
				t.Errorf("link runnable = %v, want %v", got, tc.wantRunnable)
			}
			joined := strings.Join(installPlan.Notices, "\n")
			if tc.wantNotice == "" && len(installPlan.Notices) != 0 {
				t.Errorf("unexpected notices: %v", installPlan.Notices)
			}
			if tc.wantNotice != "" && !strings.Contains(joined, tc.wantNotice) {
				t.Errorf("notices = %q, want one naming %q", joined, tc.wantNotice)
			}
			if len(installPlan.Blocking) != 0 {
				t.Errorf("blocking = %v, a GPU env problem must not block the install", installPlan.Blocking)
			}
			for _, ps := range installPlan.Steps {
				if _, isLink := ps.Step.(plan.SymlinkStep); isLink {
					continue
				}
				if c, ok := ps.Step.(plan.CommandStep); ok && strings.Contains(strings.Join(c.Argv, " "), "gpu_env.lua") {
					t.Errorf("command touches gpu_env.lua: %v", c.Argv)
				}
			}
		})
	}
}

func TestPlan_SecretsFileIsCreatedOnlyWhenMissingAndOmitsEmptyValues(t *testing.T) {
	entries := []plan.SecretEntry{{Name: "API_KEY", Value: "k1"}, {Name: "EMPTY", Value: ""}}
	cases := []struct {
		name      string
		mutate    func(f *planFixture)
		wantStep  bool
		wantNames []string
	}{
		{name: "values entered", mutate: func(f *planFixture) { f.draft.Secrets = entries }, wantStep: true, wantNames: []string{"API_KEY"}},
		{name: "no names configured", mutate: func(*planFixture) {}, wantStep: true},
		{
			name:   "file exists",
			mutate: func(f *planFixture) { f.in.kinds[secretsPath] = plan.KindFile; f.draft.Secrets = entries },
		},
		{
			name:   "owner skipped the page",
			mutate: func(f *planFixture) { f.draft.SkipSecrets = true; f.draft.Secrets = entries },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			tc.mutate(f)
			steps := stepsOfType[plan.WriteSecretsStep](rawSteps(runnable(f.plan(t).Steps)))
			if (len(steps) == 1) != tc.wantStep || len(steps) > 1 {
				t.Fatalf("runnable secrets steps = %+v, want present=%v", steps, tc.wantStep)
			}
			if !tc.wantStep {
				return
			}
			var names []string
			for _, e := range steps[0].Entries {
				names = append(names, e.Name)
			}
			if steps[0].Path != secretsPath || !slices.Equal(names, tc.wantNames) {
				t.Errorf("secrets step = path %q names %v, want %q %v", steps[0].Path, names, secretsPath, tc.wantNames)
			}
		})
	}
}

func TestPlan_ShellStepsCloneOnlyWhatIsMissingAndChangeShellOnlyWhenNotZsh(t *testing.T) {
	all := sorted(ohMyZshPath, pluginsDir+"/zsh-syntax-highlighting", pluginsDir+"/zsh-autosuggestions")
	cases := []struct {
		name       string
		mutate     func(f *planFixture)
		wantClones []string
		wantChsh   bool
	}{
		{name: "fresh machine", mutate: func(*planFixture) {}, wantClones: all, wantChsh: true},
		{
			name: "oh-my-zsh and one plugin exist",
			mutate: func(f *planFixture) {
				f.in.kinds[ohMyZshPath] = plan.KindDir
				f.in.kinds[pluginsDir+"/zsh-syntax-highlighting"] = plan.KindDir
			},
			wantClones: []string{pluginsDir + "/zsh-autosuggestions"},
			wantChsh:   true,
		},
		{name: "shell already zsh", mutate: func(f *planFixture) { f.probe.shell = "/usr/bin/zsh" }, wantClones: all},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			tc.mutate(f)
			commands := runnableCommands(f.plan(t), domain.PhaseShell)

			if got := cloneDestinations(commands); !slices.Equal(got, tc.wantClones) {
				t.Errorf("clone destinations = %v, want %v", got, tc.wantClones)
			}
			var chsh []plan.CommandStep
			for _, c := range commands {
				if c.Argv[0] == "git" {
					if !slices.Contains(c.Argv, "--depth") || c.Sudo || c.Interactive {
						t.Errorf("clone = %+v, want unattended shallow clone", c)
					}
					continue
				}
				chsh = append(chsh, c)
			}
			if (len(chsh) == 1) != tc.wantChsh || len(chsh) > 1 {
				t.Fatalf("other shell commands = %+v, want chsh=%v", chsh, tc.wantChsh)
			}
			if tc.wantChsh && (!chsh[0].Sudo || chsh[0].Interactive || !slices.Equal(chsh[0].Argv, []string{"chsh", "-s", "/usr/bin/zsh", "u"})) {
				t.Errorf("chsh step = %+v, want unattended sudo chsh -s /usr/bin/zsh u", chsh[0])
			}
		})
	}
}

func TestPlan_ServicesAreEnabledOnlyForSelectedPackagesAndDrivers(t *testing.T) {
	cases := []struct {
		name       string
		draft      Draft
		linkTarget string
		wantUnits  []string
		wantNotice bool
	}{
		{
			name:  "full with nvidia",
			draft: Draft{Profile: manifest.ProfileFull, Drivers: []string{"nvidia"}},
			wantUnits: sorted("NetworkManager.service", "docker.service", "ly.service",
				"nvidia-suspend.service", "nvidia-hibernate.service", "nvidia-resume.service"),
		},
		{
			name:      "minimal without drivers",
			draft:     Draft{Profile: manifest.ProfileMinimal},
			wantUnits: sorted("NetworkManager.service", "ly.service"),
		},
		{
			name:       "another display manager is enabled",
			draft:      Draft{Profile: manifest.ProfileMinimal},
			linkTarget: "/usr/lib/systemd/system/sddm.service",
			wantUnits:  []string{"NetworkManager.service"},
			wantNotice: true,
		},
		{
			name:       "ly is already the display manager",
			draft:      Draft{Profile: manifest.ProfileMinimal},
			linkTarget: "/usr/lib/systemd/system/ly.service",
			wantUnits:  sorted("NetworkManager.service", "ly.service"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			f.draft = tc.draft
			f.in.linkTargets[displayManagerLink] = tc.linkTarget
			installPlan := f.plan(t)

			var units []string
			for _, c := range runnableCommands(installPlan, domain.PhaseServices) {
				if len(c.Argv) < 2 || c.Argv[0] != "systemctl" || c.Argv[1] != "enable" || !c.Sudo {
					t.Errorf("services command = %+v, want sudo systemctl enable", c)
					continue
				}
				units = append(units, c.Argv[2:]...)
			}
			if got := sorted(units...); !slices.Equal(got, tc.wantUnits) {
				t.Errorf("enabled units = %v, want %v", got, tc.wantUnits)
			}
			if (len(installPlan.Notices) > 0) != tc.wantNotice {
				t.Errorf("notices = %v, want notice=%v", installPlan.Notices, tc.wantNotice)
			}
		})
	}
}

func TestPlan_ServicesAreEnabledOnceEach(t *testing.T) {
	f := newPlanFixture(t)
	f.inst.Catalog = append(f.inst.Catalog, manifest.Package{Name: "docker-rootless", Source: manifest.SourcePacman, Profiles: []string{"full"}, Service: "docker.service"})
	f.draft = Draft{Profile: manifest.ProfileFull}

	var units []string
	for _, c := range runnableCommands(f.plan(t), domain.PhaseServices) {
		units = append(units, c.Argv[2:]...)
	}

	want := sorted("NetworkManager.service", "docker.service", "ly.service")
	if !slices.Equal(units, want) {
		t.Errorf("enabled units = %v, want %v", units, want)
	}
}

func TestPlan_ThemeStateWritesNameAndVariantWithNadirDefault(t *testing.T) {
	cases := []struct {
		name        string
		variant     string
		wantVariant string
	}{
		{name: "default variant", wantVariant: "nadir"},
		{name: "chosen variant", variant: "dusk", wantVariant: "dusk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			f.draft.ThemeVariant = tc.variant
			steps := phaseSteps(f.plan(t), domain.PhaseTheme)

			writes := stepsOfType[plan.WriteFileStep](rawSteps(steps))
			if len(steps) != 2 || len(writes) != 2 {
				t.Fatalf("theme phase = %+v, want exactly two file writes", steps)
			}
			got := map[string]string{}
			for _, w := range writes {
				if w.Privileged {
					t.Errorf("theme write %s is privileged", w.Path)
				}
				got[w.Path] = strings.TrimSpace(string(w.Content))
			}
			want := map[string]string{themeDir + "/name": "nuytek", themeDir + "/variant": tc.wantVariant}
			if len(got) != 2 || got[themeDir+"/name"] != want[themeDir+"/name"] || got[themeDir+"/variant"] != want[themeDir+"/variant"] {
				t.Errorf("theme files = %v, want %v", got, want)
			}
		})
	}
}

func TestPlan_RerunOnAnInstalledMachineSkipsCompletedEffects(t *testing.T) {
	f := newPlanFixture(t)
	f.draft = Draft{Profile: manifest.ProfileMinimal, Drivers: []string{"intel"}}
	for _, name := range []string{"hyprland", "intel-media-driver", "ly", "networkmanager", "vulkan-intel"} {
		f.probe.installed[name] = true
	}
	f.probe.shell = "/usr/bin/zsh"
	f.probe.files[pacmanConfPath] = []byte(activePacmanConf)
	f.probe.files[logindDropInPath] = []byte("[Login]\nHandlePowerKey=ignore\nHandlePowerKeyLongPress=poweroff\n")
	f.in.diff = diffReturning()
	for _, p := range []string{secretsPath, ohMyZshPath, pluginsDir + "/zsh-syntax-highlighting", pluginsDir + "/zsh-autosuggestions", paruCacheDir} {
		f.in.kinds[p] = plan.KindDir
	}
	f.in.kinds[gpuEnvLinkPath] = plan.KindLink
	f.in.linkTargets[gpuEnvLinkPath] = "gpu_env_intel.lua"

	for _, s := range rawSteps(runnable(f.plan(t).Steps)) {
		switch v := s.(type) {
		case plan.CommandStep:
			switch v.Argv[0] {
			case "pacman", "paru", "makepkg", "git", "chsh":
				t.Errorf("completed effect planned again: %v", v.Argv)
			}
		case plan.WriteFileStep:
			if strings.HasPrefix(v.Path, "/etc/") {
				t.Errorf("completed /etc write planned again: %s", v.Path)
			}
		case plan.CopyStep, plan.BackupStep, plan.SymlinkStep, plan.WriteSecretsStep:
			t.Errorf("completed effect planned again: %T", v)
		}
	}
}
