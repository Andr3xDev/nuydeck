package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var packageColumns = []column{{title: "package"}, {title: "group", priority: 1}, {title: "status", priority: 2}}

func packageRows(cells ...[3]string) []tableRow {
	rows := make([]tableRow, len(cells))
	for i, c := range cells {
		rows[i] = tableRow{cells: c[:]}
	}
	return rows
}

// startOf is the display column where cell begins in line.
func startOf(t *testing.T, line, cell string) int {
	t.Helper()
	at := strings.Index(line, cell)
	if at < 0 {
		t.Fatalf("%q does not contain %q", line, cell)
	}
	return lipgloss.Width(line[:at])
}

func TestTableLines_ColumnsStartAtTheSamePositionInHeaderAndRows(t *testing.T) {
	rows := packageRows(
		[3]string{"hyprland", "desktop", "installed"},
		[3]string{"git", "tools", "missing"},
		[3]string{"日本語", "wide", "drift"},
	)

	header, lines, _ := tableLines(theme{}, packageColumns, rows, 1, 80)

	all := append([]string{ansi.Strip(header)}, stripAll(lines)...)
	wantGroup := startOf(t, all[0], "group")
	wantStatus := startOf(t, all[0], "status")
	for i, cells := range [][2]string{{"desktop", "installed"}, {"tools", "missing"}, {"wide", "drift"}} {
		line := all[i+1]
		if got := startOf(t, line, cells[0]); got != wantGroup {
			t.Errorf("row %d: group starts at %d, want %d (%q)", i, got, wantGroup, line)
		}
		if got := startOf(t, line, cells[1]); got != wantStatus {
			t.Errorf("row %d: status starts at %d, want %d (%q)", i, got, wantStatus, line)
		}
	}
}

func TestTableLines_WideRunesAreMeasuredByDisplayWidth(t *testing.T) {
	rows := packageRows([3]string{"日本語日本", "G1", "x"}, [3]string{"ab", "G2", "y"})

	_, lines, _ := tableLines(theme{}, packageColumns, rows, 0, 80)

	lines = stripAll(lines)
	if a, b := startOf(t, lines[0], "G1"), startOf(t, lines[1], "G2"); a != b {
		t.Errorf("group starts at %d after a wide cell and %d after a narrow one, want equal", a, b)
	}
}

func TestTableLines_NarrowPanelDropsHighestPriorityColumnsFirstAndKeepsTheFirst(t *testing.T) {
	rows := packageRows([3]string{"hyprland", "desktop", "installed"})
	tests := []struct {
		name        string
		width       int
		wantTitles  []string
		wantMissing []string
	}{
		{"wide panel shows every column", 80, []string{"package", "group", "status"}, nil},
		{"status dropped first", 20, []string{"package", "group"}, []string{"status", "installed"}},
		{"group dropped next", 12, []string{"package"}, []string{"group", "desktop", "status", "installed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, lines, _ := tableLines(theme{}, packageColumns, rows, 0, tt.width)

			shown := ansi.Strip(header) + "\n" + strings.Join(stripAll(lines), "\n")
			for _, want := range tt.wantTitles {
				if !strings.Contains(shown, want) {
					t.Errorf("width %d: %q missing from\n%s", tt.width, want, shown)
				}
			}
			for _, gone := range tt.wantMissing {
				if strings.Contains(shown, gone) {
					t.Errorf("width %d: %q should be dropped\n%s", tt.width, gone, shown)
				}
			}
			if !strings.Contains(shown, "hyprland") {
				t.Errorf("width %d: the first column was dropped\n%s", tt.width, shown)
			}
		})
	}
}

func TestTableLines_LastColumnIsElasticAndTruncatedWithAMarker(t *testing.T) {
	const long = "a-very-long-description-that-cannot-fit-in-the-panel"
	cols := []column{{title: "name"}, {title: "description"}}
	rows := []tableRow{{cells: []string{"x", long}}, {cells: []string{"y", "short"}}}

	_, wide, _ := tableLines(theme{}, cols, rows, 1, 100)
	_, narrow, _ := tableLines(theme{}, cols, rows, 1, 30)

	if got := ansi.Strip(wide[0]); !strings.Contains(got, long) {
		t.Errorf("wide panel line = %q, want the whole description", got)
	}
	got := ansi.Strip(narrow[0])
	if !strings.HasSuffix(got, "…") || lipgloss.Width(got) != 30 {
		t.Errorf("narrow line = %q (width %d), want a description truncated with … that fills the panel", got, lipgloss.Width(got))
	}
}

func TestTableLines_NoLineIsWiderThanThePanel(t *testing.T) {
	rows := packageRows(
		[3]string{"hyprland-with-a-long-name", "desktop-environment", "installed"},
		[3]string{"日本語日本語", "グループ", "missing"},
	)
	for width := 1; width <= 70; width++ {
		header, lines, _ := tableLines(theme{}, packageColumns, rows, 0, width)
		for _, l := range append([]string{header}, lines...) {
			if got := lipgloss.Width(l); got > width {
				t.Fatalf("width %d: line %q is %d wide", width, ansi.Strip(l), got)
			}
		}
	}
}

func TestTableLines_CellsCannotCarryEscapesOrBreakTheRow(t *testing.T) {
	rows := packageRows(
		[3]string{"a\x1b[31mred\x1b]8;;http://x\x1b\\", "g", "s"},
		[3]string{"two\nlines", "g", "s"},
	)

	header, lines, _ := tableLines(theme{}, packageColumns, rows, 0, 80)

	if len(lines) != 2 {
		t.Fatalf("lines = %d, want one per row", len(lines))
	}
	for _, l := range append([]string{header}, lines...) {
		if strings.ContainsAny(l, "\x1b\n") {
			t.Errorf("line %q carries an escape or newline", l)
		}
	}
}

func TestTableLines_HeaderIsOutsideTheRowsAndTheCursorStaysOnData(t *testing.T) {
	var cells [][3]string
	for i := range 30 {
		cells = append(cells, [3]string{"pkg" + string(rune('a'+i%26)) + string(rune('a'+i/26)), "g", "s"})
	}
	rows := packageRows(cells...)

	for _, cursor := range []int{0, 15, 29} {
		header, lines, cursorAt := tableLines(theme{}, packageColumns, rows, cursor, 80)

		if len(lines) != len(rows) {
			t.Fatalf("lines = %d, want one per data row (%d): the header must not be among them", len(lines), len(rows))
		}
		if cursorAt != cursor {
			t.Errorf("cursor %d: cursorAt = %d, want the same data row", cursor, cursorAt)
		}
		if !strings.HasPrefix(ansi.Strip(lines[cursorAt]), "> ") {
			t.Errorf("cursor %d: line %q is not the selected one", cursor, ansi.Strip(lines[cursorAt]))
		}
		window := windowAround(lines, cursorAt, 5)
		if len(window) != 5 || !slicesContain(window, lines[cursorAt]) {
			t.Errorf("cursor %d: window of 5 = %d lines, cursor row visible = %v", cursor, len(window), slicesContain(window, lines[cursorAt]))
		}
		if !strings.Contains(ansi.Strip(header), "package") {
			t.Errorf("header = %q, want the column titles", ansi.Strip(header))
		}
	}
}

func stripAll(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Strip(l)
	}
	return out
}

func slicesContain(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}
