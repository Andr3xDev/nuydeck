package ui

import (
	"context"
	"fmt"
	"image/color"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const (
	finalNotices = "Group changes such as docker need a new login. The logind power-key change and the theme apply after the reboot or the first Hyprland login."
	gpuFallback  = "GPU detection failed or found no known GPU: every driver is proposed. Uncheck the ones you do not need."

	summaryChrome = 4
	runChrome     = 4
)

var pageNames = []string{"profile", "hardware", "selection", "secrets", "summary", "run"}

func paint(c color.Color, text string) string {
	return lipgloss.NewStyle().Foreground(c).Render(text)
}

func bold(text string) string { return lipgloss.NewStyle().Bold(true).Render(text) }

func (s *Install) View(width, height int) string {
	return renderPanel(width, height, s.body(width, max(height-2*panelPadY, 1)))
}

func (s *Install) breadcrumb() string {
	parts := make([]string, len(pageNames))
	for i, name := range pageNames {
		parts[i] = name
		if installPage(i) == s.page {
			parts[i] = bold("[" + name + "]")
		}
	}
	return strings.Join(parts, " > ")
}

func (s *Install) hint(text string) string { return paint(s.palette.TextMuted, text) }

func (s *Install) body(width, height int) string {
	head := s.breadcrumb() + "\n\n"
	switch s.page {
	case pageProfile:
		return head + s.profileView()
	case pageHardware:
		return head + s.hardwareView(width)
	case pageSelection:
		return head + s.selectionView(height-2)
	case pageSecrets:
		return head + s.secretsView(width)
	case pageSummary:
		return head + s.summaryView(width, height-2)
	}
	return head + s.runView(width, height-2)
}

func cursorMark(on bool) string {
	if on {
		return "> "
	}
	return "  "
}

func checkbox(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func (s *Install) profileView() string {
	descriptions := map[string]string{
		domain.ProfileMinimal: "the essential packages",
		domain.ProfileFull:    "every packaged tool",
		app.ProfileCustom:     "mark packages one by one",
	}
	lines := []string{bold("Choose a profile")}
	for i, p := range profileChoices {
		lines = append(lines, fmt.Sprintf("%s%-8s %s", cursorMark(i == s.cursor), p, descriptions[p]))
	}
	return strings.Join(append(lines, "", s.hint("up/down move, enter next")), "\n")
}

func (s *Install) hardwareView(width int) string {
	lines := []string{bold("Hardware drivers")}
	switch {
	case s.hwLoading:
		return lines[0] + "\n" + s.hint("Detecting GPUs...")
	case !s.detected:
		lines = append(lines, paint(s.palette.Warning, wrapToPanel(width, gpuFallback)))
	}
	for i, d := range s.deps.Drivers {
		on := slices.Contains(s.draft.Drivers, d.ID)
		lines = append(lines, fmt.Sprintf("%s%s %s (%s)", cursorMark(i == s.cursor), checkbox(on), d.ID, d.VendorID))
	}
	skip := len(s.deps.Drivers)
	lines = append(lines, fmt.Sprintf("%s%s skip the GPU env link (gpu_env.lua)", cursorMark(s.cursor == skip), checkbox(s.draft.SkipGPUEnvLink)))
	return strings.Join(append(lines, "", s.hint("up/down move, space toggles, enter next, esc back")), "\n")
}

func (s *Install) selectionView(height int) string {
	lines := []string{bold("Packages to install")}
	visible := max(height-3, 1)
	start := max(min(s.cursor-visible/2, len(s.rows)-visible), 0)
	for i := start; i < min(start+visible, len(s.rows)); i++ {
		row := s.rows[i]
		if row.name == "" {
			group := row.group
			if group == "" {
				group = "other"
			}
			lines = append(lines, cursorMark(i == s.cursor)+bold(group))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s  %s %s", cursorMark(i == s.cursor), checkbox(s.selected[row.name]), row.name))
	}
	return strings.Join(append(lines, s.hint("space toggles a package or a whole group, enter next, esc back")), "\n")
}

func (s *Install) secretsView(width int) string {
	lines := []string{bold("Secrets")}
	switch {
	case s.secretsExist:
		lines = append(lines, wrapToPanel(width, s.deps.SecretsPath+" already exists and is left as is."), "", s.hint("enter next, esc back"))
	case s.secretPrompting():
		lines = append(lines,
			fmt.Sprintf("Value for %s (%d of %d). Nothing is shown while you type.", s.deps.SecretNames[s.secretIdx], s.secretIdx+1, len(s.deps.SecretNames)),
			"> "+s.secretInput.View(), "",
			s.hint("enter submits (empty omits), ctrl+s skips the file, esc back"))
	}
	return strings.Join(lines, "\n")
}

// showSummary renders the plan through the preview runner: names and paths only, never secret values.
func (s *Install) showSummary() {
	var lines []string
	for i, ps := range s.plan.Steps {
		label := stepLabel(ps.Step)
		if ps.Skip != "" {
			lines = append(lines, fmt.Sprintf("%2d. [skipped: %s] %s", i+1, ps.Skip, label))
			continue
		}
		detail := label
		if s.deps.Preview != nil {
			if res := s.deps.Preview.Run(context.Background(), ps.Step, nil); res.Detail != "" {
				detail = res.Detail
			}
		}
		lines = append(lines, fmt.Sprintf("%2d. %s", i+1, detail))
	}
	for _, n := range s.plan.Notices {
		lines = append(lines, "", paint(s.palette.Warning, "notice: "+n))
	}
	for _, err := range s.plan.Blocking {
		lines = append(lines, "", paint(s.palette.Error, "blocking: "+err.Error()))
	}
	lines = append(lines, "", finalNotices)
	s.summary.SetContentLines(lines)
	s.summary.GotoTop()
}

func (s *Install) summaryView(width, height int) string {
	switch {
	case s.planning:
		return s.hint("Planning...")
	case s.planErr != nil:
		return paint(s.palette.Error, wrapToPanel(width, "Plan failed: "+s.planErr.Error())) + "\n\n" + s.hint("esc back")
	}
	s.summary.SetWidth(panelInnerWidth(width))
	s.summary.SetHeight(max(height-summaryChrome, 1))
	footer := s.hint("up/down scroll, enter starts the install, esc back")
	if !s.canConfirm() {
		footer = paint(s.palette.Error, "The plan cannot run.") + " " + s.hint("esc back")
	}
	return bold("Summary") + "\n" + s.summary.View() + "\n\n" + footer
}

func stepLabel(step domain.Step) string {
	switch s := step.(type) {
	case domain.CommandStep:
		return s.Label
	case domain.WriteFileStep:
		return "write " + s.Path
	case domain.SymlinkStep:
		return "link " + s.Path
	case domain.WriteSecretsStep:
		return "create " + s.Path
	case domain.CopyStep:
		return "copy " + string(s.Entry.Root) + ":" + s.Entry.Path
	case domain.BackupStep:
		return "back up overwritten files"
	case domain.AppendLogStep:
		return "record install log"
	}
	return fmt.Sprintf("%T", step)
}

func (s *Install) statusMark(st stepStatus) string {
	switch st {
	case statusRunning:
		return "[>]"
	case statusDone:
		return "[x]"
	case statusSkipped:
		return "[-]"
	case statusFailed:
		return paint(s.palette.Error, "[!]")
	}
	return "[ ]"
}

func (s *Install) runView(width, height int) string {
	listHeight := min(len(s.plan.Steps), max(height/3, 3))
	start := max(min(s.current-listHeight/2, len(s.plan.Steps)-listHeight), 0)
	var lines []string
	for i := start; i < start+listHeight; i++ {
		lines = append(lines, s.statusMark(s.statuses[i])+" "+stepLabel(s.plan.Steps[i].Step))
	}

	s.logView.SetWidth(panelInnerWidth(width))
	s.logView.SetHeight(max(height-listHeight-runChrome, 1))
	if s.follow {
		s.logView.GotoBottom()
	}
	return strings.Join(lines, "\n") + "\n\n" + s.logView.View() + "\n\n" + s.runStatus(width)
}

func (s *Install) runStatus(width int) string {
	switch {
	case s.awaitAck:
		return paint(s.palette.Warning, "Install complete. Press Enter to reboot, ctrl+c to quit without rebooting.") + "\n" + wrapToPanel(width, finalNotices)
	case s.running:
		return s.hint("Installing... ctrl+c cancels and quits.")
	case s.runErr != nil:
		return paint(s.palette.Error, wrapToPanel(width, "Install stopped: "+s.runErr.Error())) + "\n" + s.hint("esc back to the summary, ctrl+c quits.")
	}
	return "Done. Nothing was rebooted.\n" + wrapToPanel(width, finalNotices) + "\n" + s.hint("esc back to the summary, ctrl+c quits.")
}
