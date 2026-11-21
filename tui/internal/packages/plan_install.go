package packages

import (
	"fmt"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// PlanInstall installs the missing rows with pacman; AUR rows are dropped with one notice each.
// No -u: the Packages screen installs, it does not upgrade the system.
func PlanInstall(rows []Row) (plan.Plan, []string) {
	var names, notices []string
	missing := 0
	for _, r := range rows {
		if r.Status != StatusMissing {
			continue
		}
		missing++
		if r.Source == manifest.SourceAUR {
			notices = append(notices, fmt.Sprintf("%s is an AUR package, install it with paru", r.Name))
			continue
		}
		names = append(names, r.Name)
	}
	if missing == 0 {
		notices = append(notices, "no marked package is missing")
	}
	return pacmanPlan("install packages", "Install the marked packages?", "", []string{"-S", "--needed", "--noconfirm"}, names), notices
}

// PlanUninstall removes the installed rows with pacman -Rns; the manifest is left as it is.
func PlanUninstall(rows []Row) (plan.Plan, []string) {
	var names []string
	for _, r := range rows {
		if r.Status == StatusInstalled {
			names = append(names, r.Name)
		}
	}
	if len(names) == 0 {
		return plan.Plan{}, []string{"no marked package is installed"}
	}
	return pacmanPlan("uninstall packages", "Uninstall the marked packages? The manifest is not edited, they stay listed as missing.", "Packages and their unneeded dependencies are removed from this machine. Continue?", []string{"-Rns", "--noconfirm"}, names), nil
}

func pacmanPlan(title, confirm, extraConfirm string, flags, names []string) plan.Plan {
	if len(names) == 0 {
		return plan.Plan{}
	}
	argv := append(append([]string{"pacman"}, flags...), names...)
	steps := append(plan.PrivilegeSteps(), plan.CommandStep{Label: title, Argv: argv, Sudo: true})
	return plan.Plan{Title: title, Steps: steps, Confirm: confirm, ExtraConfirm: extraConfirm}
}
