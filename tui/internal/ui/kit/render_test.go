package kit

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderBox_LongTitleKeepsEveryLineAtWidth(t *testing.T) {
	const w = 20
	out := RenderBox(Theme{}, strings.Repeat("T", 100), []string{"x"}, w, 5, true)

	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Errorf("box has %d lines, want 5", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != w {
			t.Errorf("line %d width = %d, want %d: %q", i, got, w, line)
		}
	}
}

func TestContentWidth_ClampsAtZero(t *testing.T) {
	for _, tc := range []struct{ w, want int }{{20, 17}, {3, 0}, {0, 0}, {-5, 0}} {
		if got := ContentWidth(tc.w); got != tc.want {
			t.Errorf("contentWidth(%d) = %d, want %d", tc.w, got, tc.want)
		}
	}
}

func TestTruncate_CutsToWidthWithEllipsis(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"fits unchanged", "short", 10, "short"},
		{"exact fit unchanged", "short", 5, "short"},
		{"cut ends with ellipsis", "abcdefghij", 5, "abcd…"},
		{"wide runes stay within width", "日本語日本語", 5, "日本…"},
		{"zero width is empty", "abc", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Truncate(tc.in, tc.width)
			if got != tc.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
			if lipgloss.Width(got) > tc.width {
				t.Fatalf("truncate(%q, %d) width = %d", tc.in, tc.width, lipgloss.Width(got))
			}
		})
	}
}

func TestWrap_StripsTerminalSequences(t *testing.T) {
	in := "a\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\ b\x1bPq data\x1b\\ \u009b31m c‮ d"
	for _, line := range Wrap(in, 80) {
		if strings.ContainsAny(line, "\x1b\u009b‮") {
			t.Errorf("control character survived: %q", line)
		}
	}
}

func TestWrap_LinesFitWidth(t *testing.T) {
	const w = 10
	for _, line := range Wrap(strings.Repeat("word ", 20), w) {
		if got := lipgloss.Width(line); got > w {
			t.Errorf("wrapped line width = %d, want <= %d: %q", got, w, line)
		}
	}
}

var sanitizeCases = []struct{ name, in, wantKeepNewline string }{
	{"ESC and C1", "a\x1b]8;;x\x1b\\b\u009bc", "a]8;;x\\bc"},
	{"C0 control", "a\x07b\x00c\td", "abcd"},
	{"C1 range edges", "a\u0080b\u009fc", "abc"},
	{"bidi override", "a‪b‮c", "abc"},
	{"bidi isolate", "a⁦b⁩c", "abc"},
	{"newline", "a\nb", "a\nb"},
}

func TestSanitize_DropsTerminalAndBidiControls(t *testing.T) {
	for _, tc := range sanitizeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitize(tc.in); got != tc.wantKeepNewline {
				t.Fatalf("sanitize(%q) = %q, want %q", tc.in, got, tc.wantKeepNewline)
			}
		})
	}
}

func TestSanitizeLine_AlsoDropsNewline(t *testing.T) {
	for _, tc := range sanitizeCases {
		t.Run(tc.name, func(t *testing.T) {
			want := strings.ReplaceAll(tc.wantKeepNewline, "\n", "")
			if got := SanitizeLine(tc.in); got != want {
				t.Fatalf("sanitizeLine(%q) = %q, want %q", tc.in, got, want)
			}
		})
	}
}

func numberedLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	return lines
}

func TestWindowAround_KeepsLineVisible(t *testing.T) {
	lines := numberedLines(20)
	for _, h := range []int{1, 3, 5, 20, 30} {
		for at := range lines {
			win := WindowAround(lines, at, h)
			if len(win) > h {
				t.Errorf("h=%d at=%d: window has %d lines", h, at, len(win))
			}
			if !slices.Contains(win, lines[at]) {
				t.Errorf("h=%d at=%d: %q not in window %v", h, at, lines[at], win)
			}
		}
	}
}

func TestWindowAround_ZeroHeightIsEmpty(t *testing.T) {
	if win := WindowAround(numberedLines(5), 2, 0); len(win) != 0 {
		t.Fatalf("window = %v, want empty", win)
	}
}

func TestScroller_WindowClampsOffsetToContent(t *testing.T) {
	lines := numberedLines(10)
	s := Scroller{}

	s.By(100)
	if win := s.Window(lines, 4); !slices.Equal(win, lines[6:]) {
		t.Errorf("window after overscroll = %v, want last 4 lines", win)
	}

	s.By(-100)
	if win := s.Window(lines, 4); !slices.Equal(win, lines[:4]) {
		t.Errorf("window after underscroll = %v, want first 4 lines", win)
	}
}

func TestScroller_ResetReturnsToTop(t *testing.T) {
	lines := numberedLines(10)
	s := Scroller{}
	s.By(5)
	s.Reset()

	if win := s.Window(lines, 3); !slices.Equal(win, lines[:3]) {
		t.Fatalf("window after reset = %v, want first 3 lines", win)
	}
}

func TestListLines_ReturnsLineWhereCursorRowStarts(t *testing.T) {
	const w = 12
	rows := []ListRow{
		{Text: strings.Repeat("long ", 6)},
		{Text: "cursor"},
		{Text: "after", Muted: true},
	}

	lines, cursorAt := ListLines(Theme{}, rows, 1, w)

	if cursorAt <= 0 || cursorAt >= len(lines) {
		t.Fatalf("cursorAt = %d of %d lines; the wrapped first row must precede it", cursorAt, len(lines))
	}
	if !strings.Contains(lines[cursorAt], "> cursor") {
		t.Errorf("line %d = %q, want the cursor row", cursorAt, lines[cursorAt])
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > w {
			t.Errorf("line %d width = %d, want <= %d: %q", i, got, w, line)
		}
	}
}

func TestListLines_StripsControlsFromRowText(t *testing.T) {
	lines, _ := ListLines(Theme{}, []ListRow{{Text: "a\x1b]8;;x\x1b\\b‮c"}}, 0, 40)

	for _, line := range lines {
		if strings.ContainsAny(line, "\x1b‮") {
			t.Errorf("control character survived: %q", line)
		}
	}
}
