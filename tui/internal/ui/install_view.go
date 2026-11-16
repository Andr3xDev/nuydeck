package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	finalNotices = "Group changes such as docker need a new login. The logind power-key change and the theme apply after the reboot or the first Hyprland login."
	gpuFallback  = "GPU detection failed or found no known GPU: every driver is proposed. Uncheck the ones you do not need."

	markWidth     = 3 // "[x]"
	summaryChrome = 4
	runChrome     = 4
)

var pageNames = []string{"profile", "hardware", "selection", "secrets", "summary", "run"}

func (s *Install) View(width, height int) string {
	title := fmt.Sprintf("Install %d/%d %s", int(s.page)+1, len(pageNames), pageNames[s.page])
	return screenBox(s.palette, title, s.body(width, innerHeight(height)), width, height)
}

// hint wraps muted help text to the content width.
func (s *Install) hint(text string, cw int) []string {
	return styled(s.palette.TextMuted, wrap(text, cw))
}

func (s *Install) body(width, height int) []string {
	cw := contentWidth(width)
	switch s.page {
	case pageProfile:
		return s.profileView(width)
	case pageHardware:
		return s.hardwareView(width)
	case pageSelection:
		return s.selectionView(cw, height)
	case pageSecrets:
		return s.secretsView(cw)
	case pageSummary:
		return s.summaryView(cw, height)
	}
	return s.runView(cw, height)
}

func (s *Install) profileView(width int) []string {
	cw := contentWidth(width)
	descriptions := map[string]string{
		manifest.ProfileMinimal: "the essential packages",
		manifest.ProfileFull:    "every packaged tool",
		install.ProfileCustom:   "mark packages one by one",
	}
	rows := make([]listRow, len(profileChoices))
	for i, p := range profileChoices {
		rows[i] = listRow{text: p}
		if width >= wideRowWidth {
			rows[i].text += "  " + descriptions[p]
		}
	}
	list, _ := listLines(newTheme(s.palette), rows, s.cursor, cw)
	lines := append([]string{bold("Choose a profile")}, list...)
	return append(append(lines, ""), s.hint("up/down move, enter next", cw)...)
}

func (s *Install) hardwareView(width int) []string {
	cw := contentWidth(width)
	lines := []string{bold("Hardware drivers")}
	switch {
	case s.hwLoading:
		return append(lines, s.hint("Detecting GPUs...", cw)...)
	case !s.detected:
		lines = append(lines, styled(s.palette.Warning, wrap(gpuFallback, cw))...)
	}
	rows := make([]listRow, 0, len(s.deps.Drivers)+1)
	for _, d := range s.deps.Drivers {
		rows = append(rows, listRow{text: fmt.Sprintf("%s %s (%s)", checkbox(slices.Contains(s.draft.Drivers, d.ID)), d.ID, d.VendorID)})
	}
	rows = append(rows, listRow{text: checkbox(s.draft.SkipGPUEnvLink) + " skip the GPU env link (gpu_env.lua)"})
	list, _ := listLines(newTheme(s.palette), rows, s.cursor, cw)
	lines = append(lines, list...)
	return append(append(lines, ""), s.hint("up/down move, space toggles, enter next, esc back", cw)...)
}

func (s *Install) selectionView(cw, height int) []string {
	lines := []string{bold("Packages to install")}
	hint := s.hint("space toggles a package or a whole group, enter next, esc back", cw)
	rows := make([]listRow, len(s.rows))
	for i, row := range s.rows {
		switch {
		case row.name != "":
			rows[i] = listRow{text: "  " + checkbox(s.selected[row.name]) + " " + row.name}
		case row.group == "":
			rows[i] = listRow{text: "other"}
		default:
			rows[i] = listRow{text: row.group}
		}
	}
	list, at := listLines(newTheme(s.palette), rows, s.cursor, cw)
	lines = append(lines, windowAround(list, at, max(height-len(lines)-len(hint), 1))...)
	return append(lines, hint...)
}

func (s *Install) secretsView(cw int) []string {
	lines := []string{bold("Secrets")}
	switch {
	case s.secretsExist:
		lines = append(lines, wrap(s.deps.SecretsPath+" already exists and is left as is.", cw)...)
		lines = append(append(lines, ""), s.hint("enter next, esc back", cw)...)
	case s.secretPrompting():
		lines = append(lines, wrap(fmt.Sprintf("Value for %s (%d of %d). Nothing is shown while you type.", s.deps.SecretNames[s.secretIdx], s.secretIdx+1, len(s.deps.SecretNames)), cw)...)
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
			lines = append(lines, sanitizeLine(fmt.Sprintf("%2d. [skipped: %s] %s", i+1, ps.Skip, label)))
			continue
		}
		detail := label
		if s.deps.Preview != nil {
			if res := s.deps.Preview.Run(context.Background(), ps.Step, nil); res.Detail != "" {
				detail = res.Detail
			}
		}
		lines = append(lines, sanitizeLine(fmt.Sprintf("%2d. %s", i+1, detail)))
	}
	for _, n := range s.plan.Notices {
		lines = append(lines, "", paint(s.palette.Warning, sanitizeLine("notice: "+n)))
	}
	for _, err := range s.plan.Blocking {
		lines = append(lines, "", paint(s.palette.Error, sanitizeLine("blocking: "+err.Error())))
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
		lines := styled(s.palette.Error, wrap("Plan failed: "+s.planErr.Error(), cw))
		return append(append(lines, ""), s.hint("esc back", cw)...)
	}
	s.summary.SetWidth(max(cw, 1))
	s.summary.SetHeight(max(height-summaryChrome, 1))
	footer := s.hint("up/down scroll, enter starts the install, esc back", cw)
	if !s.canConfirm() {
		footer = []string{paint(s.palette.Error, "The plan cannot run.") + " " + paint(s.palette.TextMuted, "esc back")}
	}
	lines := append([]string{bold("Summary")}, strings.Split(s.summary.View(), "\n")...)
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

func (s *Install) runView(cw, height int) []string {
	listHeight := min(len(s.plan.Steps), max(height/3, 3))
	steps := make([]string, len(s.plan.Steps))
	for i, step := range s.plan.Steps {
		steps[i] = s.statusMark(s.statuses[i]) + " " + truncate(sanitizeLine(stepLabel(step.Step)), max(cw-markWidth-1, 0))
	}
	lines := windowAround(steps, s.current, listHeight)

	s.logView.SetWidth(max(cw, 1))
	s.logView.SetHeight(max(height-listHeight-runChrome, 1))
	if s.follow {
		s.logView.GotoBottom()
	}
	lines = append(lines, "")
	lines = append(lines, strings.Split(s.logView.View(), "\n")...)
	return append(append(lines, ""), s.runStatus(cw)...)
}

func (s *Install) runStatus(cw int) []string {
	switch {
	case s.awaitAck:
		return append(styled(s.palette.Warning, wrap("Install complete. Press Enter to reboot, ctrl+c to quit without rebooting.", cw)), wrap(finalNotices, cw)...)
	case s.running:
		return s.hint("Installing... ctrl+c cancels and quits.", cw)
	case s.runErr != nil:
		return append(styled(s.palette.Error, wrap("Install stopped: "+s.runErr.Error(), cw)), s.hint("esc back to the summary, ctrl+c quits.", cw)...)
	}
	lines := append([]string{"Done. Nothing was rebooted."}, wrap(finalNotices, cw)...)
	return append(lines, s.hint("esc back to the summary, ctrl+c quits.", cw)...)
}
