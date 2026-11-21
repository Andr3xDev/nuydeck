package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const tooSmall = "Terminal too small"

func sizedRoot(deps Deps, w, h int) *Root {
	r := NewRoot("", false, deps)
	r.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return r
}

func renderedLines(r *Root) []string {
	return strings.Split(ansi.Strip(r.View().Content), "\n")
}

// sidebarColumns is the sidebar block width, found from its right border.
func sidebarColumns(r *Root) int {
	line := renderedLines(r)[0]
	return lipgloss.Width(line[:strings.Index(line, "│")]) + 1
}

func TestRootFitsTheTerminal(t *testing.T) {
	const warning = "state file is corrupt and was ignored; the next sync starts from an empty history so nothing is lost"
	for _, size := range []struct{ w, h int }{{80, 24}, {40, 8}} {
		for _, stateWarning := range []string{"", warning} {
			r := sizedRoot(Deps{StateWarning: stateWarning}, size.w, size.h)
			r.selectSection(2)

			lines := renderedLines(r)
			if len(lines) > size.h {
				t.Errorf("%dx%d warning=%t: %d lines, want at most %d", size.w, size.h, stateWarning != "", len(lines), size.h)
			}
			for i, line := range lines {
				if got := lipgloss.Width(line); got > size.w {
					t.Errorf("%dx%d warning=%t: line %d is %d cells wide: %q", size.w, size.h, stateWarning != "", i, got, line)
				}
			}
			if last := lines[len(lines)-1]; !strings.Contains(last, "quit") {
				t.Errorf("%dx%d warning=%t: last line is not the keybar: %q", size.w, size.h, stateWarning != "", last)
			}
		}
	}
}

func TestRootSidebarShowsTheTitleFirstAndConfigOnTheLastRow(t *testing.T) {
	const sidebarWidth = 30
	for _, size := range []struct{ w, h int }{{100, 24}, {40, 8}} {
		r := sizedRoot(Deps{}, size.w, size.h)
		rows := make([]string, 0, size.h)
		for _, line := range renderedLines(r)[:size.h-1] {
			rows = append(rows, strings.TrimSpace(line[:strings.Index(line, "│")]))
		}

		if rows[0] != "NUYDECK" || rows[1] != "Dashboard" || rows[len(rows)-1] != "Config" {
			t.Errorf("%dx%d: sidebar rows = %q, want NUYDECK first, Dashboard under it and Config last", size.w, size.h, rows)
		}
	}
	if got := sidebarColumns(sizedRoot(Deps{}, 100, 24)); got != sidebarWidth {
		t.Errorf("sidebar width = %d, want %d", got, sidebarWidth)
	}
}

func TestRootSidebarKeepsTheSelectionVisibleWhenSectionsDoNotFit(t *testing.T) {
	r := sizedRoot(fullDeps(), 100, 9)
	r.selectSection(6)

	if view := ansi.Strip(r.View().Content); !strings.Contains(view, "Services") || !strings.Contains(view, "Config") {
		t.Errorf("sidebar lost the selected section or Config:\n%s", view)
	}
}

func TestRootBelowMinimumSizeShowsOnlyTheTooSmallMessage(t *testing.T) {
	for _, size := range []struct{ w, h int }{{39, 8}, {40, 7}} {
		view := ansi.Strip(sizedRoot(Deps{}, size.w, size.h).View().Content)
		if !strings.Contains(view, tooSmall) {
			t.Errorf("%dx%d: view lacks %q: %q", size.w, size.h, tooSmall, view)
		}
	}
	if view := ansi.Strip(sizedRoot(Deps{}, 40, 8).View().Content); strings.Contains(view, tooSmall) {
		t.Errorf("40x8 must be usable, got %q", view)
	}
}

func TestRootKeepsStateAcrossATooSmallResize(t *testing.T) {
	r := sizedRoot(Deps{}, 100, 30)
	r.selectSection(2)
	r.focus = focusPanel

	r.Update(tea.WindowSizeMsg{Width: 30, Height: 5})
	if view := ansi.Strip(r.View().Content); !strings.Contains(view, tooSmall) {
		t.Fatalf("30x5 view lacks %q: %q", tooSmall, view)
	}
	r.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := ansi.Strip(r.View().Content)
	if strings.Contains(view, tooSmall) || r.selected != 2 || r.focus != focusPanel {
		t.Errorf("after grow: selected=%d focus=%v too-small=%t, want section 2 with panel focus restored", r.selected, r.focus, strings.Contains(view, tooSmall))
	}
}

func TestRootNoticeBandShowsTheStateWarningOnEverySection(t *testing.T) {
	const warning = "state file is corrupt and was ignored"
	r := sizedRoot(Deps{StateWarning: warning}, 100, 24)
	r.selectSection(2)

	view := ansi.Strip(r.View().Content)
	if !strings.Contains(view, warning) {
		t.Errorf("Backups view lacks the state warning:\n%s", view)
	}
	lines := renderedLines(r)
	if len(lines) != 24 || !strings.Contains(lines[len(lines)-1], "quit") {
		t.Errorf("band must shrink only the panel: %d lines, last %q", len(lines), lines[len(lines)-1])
	}
}

func TestRootNoticeBandCarriesPlanNoticesAndShowsTheStateWarningOnce(t *testing.T) {
	const (
		warning = "state file is corrupt and was ignored"
		notice  = "skipped: source missing: home/.gone"
	)
	p := selectedPlan()
	p.Notices = []string{notice}
	r := sizedRoot((&fakeSyncDeps{plan: p}).deps(), 100, 24)
	r.svc.StateWarning = warning

	r.selectSection(0)
	if got := strings.Count(ansi.Strip(r.View().Content), warning); got != 1 {
		t.Errorf("Dashboard shows the state warning %d times, want 1", got)
	}

	s, i, _ := sectionOf[*Sync](r)
	r.selectSection(i)
	runCmds(s, s.startPlan(plan.Push))
	view := ansi.Strip(r.View().Content)
	if got := strings.Count(view, warning); got != 1 {
		t.Errorf("Sync shows the state warning %d times, want 1", got)
	}
	if got := strings.Count(view, notice); got != 1 {
		t.Errorf("Sync shows the plan notice %d times, want 1 (in the band)", got)
	}
	lines := renderedLines(r)
	if len(lines) != 24 || !strings.Contains(lines[len(lines)-1], "quit") {
		t.Errorf("band must shrink only the panel: %d lines, last %q", len(lines), lines[len(lines)-1])
	}
}

func TestRootNoticeBandTruncatesWithAMoreLine(t *testing.T) {
	p := selectedPlan()
	p.Notices = []string{"first notice", "second notice", "third notice"}
	r := sizedRoot((&fakeSyncDeps{plan: p}).deps(), 100, 24)
	s, i, _ := sectionOf[*Sync](r)
	r.selectSection(i)
	runCmds(s, s.startPlan(plan.Push))

	view := ansi.Strip(r.View().Content)
	if !strings.Contains(view, "first notice") || !strings.Contains(view, "+2 more") || strings.Contains(view, "second notice") {
		t.Errorf("band must show the first notice and a +2 more line:\n%s", view)
	}
	lines := renderedLines(r)
	if len(lines) != 24 || !strings.Contains(lines[len(lines)-1], "quit") {
		t.Errorf("band must keep the keybar last: %d lines, last %q", len(lines), lines[len(lines)-1])
	}
}

func TestRootSidebarWidthIsCappedAtThirtyAndScalesDown(t *testing.T) {
	for _, tc := range []struct{ w, want int }{{100, 30}, {50, 20}} {
		if got := sidebarColumns(sizedRoot(Deps{}, tc.w, 24)); got != tc.want {
			t.Errorf("sidebar at %d columns = %d, want %d", tc.w, got, tc.want)
		}
	}
}
