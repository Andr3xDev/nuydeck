package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
)

const (
	finalNotices = "Group changes such as docker need a new login. The logind power-key change and the theme apply after the reboot or the first Hyprland login."
	gpuFallback  = "GPU detection failed or found no known GPU: every driver is proposed. Uncheck the ones you do not need."

	summaryChrome = 4
)

var pageNames = []string{"profile", "hardware", "selection", "secrets", "summary"}

func (s *Install) View(width, height int) string {
	title := fmt.Sprintf("Install %d/%d %s", int(s.page)+1, len(pageNames), pageNames[s.page])
	return screenBox(s.th, title, s.body(width, innerHeight(height)), width, height)
}

// hint wraps muted help text to the content width.
func (s *Install) hint(text string, cw int) []string {
	return styled(s.th.Muted, kit.Wrap(text, cw))
}

func (s *Install) body(width, height int) []string {
	cw := kit.ContentWidth(width)
	switch s.page {
	case pageProfile:
		return s.profileView(width)
	case pageHardware:
		return s.hardwareView(width)
	case pageSelection:
		return s.selectionView(cw, height)
	case pageSecrets:
		return s.secretsView(cw)
	}
	return s.summaryView(cw, height)
}

func (s *Install) profileView(width int) []string {
	cw := kit.ContentWidth(width)
	descriptions := map[string]string{
		manifest.ProfileMinimal: "the essential packages",
		manifest.ProfileFull:    "every packaged tool",
		install.ProfileCustom:   "mark packages one by one",
	}
	rows := make([]kit.ListRow, len(profileChoices))
	for i, p := range profileChoices {
		rows[i] = kit.ListRow{Text: p}
		if width >= wideRowWidth {
			rows[i].Text += "  " + descriptions[p]
		}
	}
	list, _ := kit.ListLines(s.th, rows, s.cursor, cw)
	lines := append([]string{s.th.Title.Render("Choose a profile")}, list...)
	return append(append(lines, ""), s.hint("up/down move, enter next", cw)...)
}

func (s *Install) hardwareView(width int) []string {
	cw := kit.ContentWidth(width)
	lines := []string{s.th.Title.Render("Hardware drivers")}
	switch {
	case s.hwLoading:
		return append(lines, s.hint("Detecting GPUs...", cw)...)
	case !s.detected:
		lines = append(lines, styled(s.th.Warning, kit.Wrap(gpuFallback, cw))...)
	}
	rows := make([]kit.ListRow, 0, len(s.deps.Drivers)+1)
	for _, d := range s.deps.Drivers {
		rows = append(rows, kit.ListRow{Text: fmt.Sprintf("%s %s (%s)", checkbox(slices.Contains(s.draft.Drivers, d.ID)), d.ID, d.VendorID)})
	}
	rows = append(rows, kit.ListRow{Text: checkbox(s.draft.SkipGPUEnvLink) + " skip the GPU env link (gpu_env.lua)"})
	list, _ := kit.ListLines(s.th, rows, s.cursor, cw)
	lines = append(lines, list...)
	return append(append(lines, ""), s.hint("up/down move, space toggles, enter next, esc back", cw)...)
}

func (s *Install) selectionView(cw, height int) []string {
	lines := []string{s.th.Title.Render("Packages to install")}
	hint := s.hint("space toggles a package or a whole group, enter next, esc back", cw)
	rows := make([]kit.ListRow, len(s.rows))
	for i, row := range s.rows {
		switch {
		case row.name != "":
			rows[i] = kit.ListRow{Text: "  " + checkbox(s.selected[row.name]) + " " + row.name}
		case row.group == "":
			rows[i] = kit.ListRow{Text: "other", Header: true}
		default:
			rows[i] = kit.ListRow{Text: row.group, Header: true}
		}
	}
	list, at := kit.ListLines(s.th, rows, s.cursor, cw)
	lines = append(lines, kit.WindowAround(list, at, max(height-len(lines)-len(hint), 1))...)
	return append(lines, hint...)
}

func (s *Install) secretsView(cw int) []string {
	lines := []string{s.th.Title.Render("Secrets")}
	switch {
	case s.secretsExist:
		lines = append(lines, kit.Wrap(s.deps.SecretsPath+" already exists and is left as is.", cw)...)
		lines = append(append(lines, ""), s.hint("enter next, esc back", cw)...)
	case s.secretPrompting():
		lines = append(lines, kit.Wrap(fmt.Sprintf("Value for %s (%d of %d). Nothing is shown while you type.", s.deps.SecretNames[s.secretIdx], s.secretIdx+1, len(s.deps.SecretNames)), cw)...)
		lines = append(lines, clip([]string{"> " + s.secretInput.View()}, cw)[0], "")
		lines = append(lines, s.hint("enter submits (empty omits), ctrl+s skips the file, esc back", cw)...)
	}
	return lines
}

// showSummary renders the plan through the preview runner: names and paths only, never secret values.
func (s *Install) showSummary() {
	var lines []string
	for i, ps := range s.plan.Steps {
		label := stepLabel(ps.Step)
		if ps.Skip != "" {
			lines = append(lines, kit.SanitizeLine(fmt.Sprintf("%2d. [skipped: %s] %s", i+1, ps.Skip, label)))
			continue
		}
		detail := label
		if s.deps.Preview != nil {
			if res := s.deps.Preview(context.Background(), ps.Step); res.Detail != "" {
				detail = res.Detail
			}
		}
		lines = append(lines, kit.SanitizeLine(fmt.Sprintf("%2d. %s", i+1, detail)))
	}
	for _, err := range s.plan.Blocking {
		lines = append(lines, "", s.th.Err.Render(kit.SanitizeLine("blocking: "+err.Error())))
	}
	lines = append(lines, "", finalNotices)
	s.summary.SetContentLines(lines)
	s.summary.GotoTop()
}

func (s *Install) summaryView(cw, height int) []string {
	switch {
	case s.planning:
		return s.hint("Planning...", cw)
	case s.planErr != nil:
		lines := styled(s.th.Err, kit.Wrap("Plan failed: "+s.planErr.Error(), cw))
		return append(append(lines, ""), s.hint("esc back", cw)...)
	}
	s.summary.SetWidth(max(cw, 1))
	s.summary.SetHeight(max(height-summaryChrome, 1))
	footer := s.hint("up/down scroll, enter starts the install, esc back", cw)
	if !s.canConfirm() {
		footer = []string{s.th.Err.Render("The plan cannot run.") + " " + s.th.Muted.Render("esc back")}
	}
	lines := append([]string{s.th.Title.Render("Summary")}, strings.Split(s.summary.View(), "\n")...)
	return append(append(lines, ""), footer...)
}

// stepLabel prefixes the kind only where the label is a bare path or name.
func stepLabel(step plan.Step) string {
	label, kind := plan.Describe(step)
	switch kind {
	case "file", "link", "secrets", "copy", "delete", "delete-backup", "package":
		return kind + " " + label
	}
	return label
}

// installFooter is the run pane's text under the output at the reboot prompt and once the install ended;
// while it runs the pane's own hint applies.
func installFooter(th kit.Theme, cw int, r *run) []string {
	switch {
	case r.awaitAck && r.question == "":
		return append(styled(th.Warning, kit.Wrap("Install complete. Press Enter to reboot, ctrl+c to quit without rebooting.", cw)), kit.Wrap(finalNotices, cw)...)
	case r.done && r.err != nil:
		return append(styled(th.Err, kit.Wrap("Install stopped: "+r.err.Error(), cw)), styled(th.Muted, kit.Wrap(runCloseText, cw))...)
	case r.done:
		lines := append([]string{"Done. Nothing was rebooted."}, kit.Wrap(finalNotices, cw)...)
		return append(lines, styled(th.Muted, kit.Wrap(runCloseText, cw))...)
	}
	return nil
}
