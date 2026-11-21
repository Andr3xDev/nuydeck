package envvars_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/envvars"
)

// varAt returns the parsed row on the given 0-based file line.
func varAt(t *testing.T, text string, line int) envvars.Var {
	t.Helper()
	for _, v := range envvars.ParseHlEnv(text) {
		if v.Line == line {
			return v
		}
	}
	t.Fatalf("no variable on line %d of %q", line, text)
	return envvars.Var{}
}

func TestToggleLine_RewritesOnlyTheCommentMarkerOfTheTargetLine(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		line       int
		want       string
		wantBefore string
		wantAfter  string
	}{
		{"comment out", "hl.env(\"A\", \"1\")\n", 0, "-- hl.env(\"A\", \"1\")\n", `hl.env("A", "1")`, `-- hl.env("A", "1")`},
		{"uncomment", "-- hl.env(\"A\", \"1\")\n", 0, "hl.env(\"A\", \"1\")\n", `-- hl.env("A", "1")`, `hl.env("A", "1")`},
		{"uncomment without space", "--hl.env(\"A\", \"1\")\n", 0, "hl.env(\"A\", \"1\")\n", `--hl.env("A", "1")`, `hl.env("A", "1")`},
		{"indent kept when commenting", "\thl.env(\"A\", \"1\")\n", 0, "\t-- hl.env(\"A\", \"1\")\n", "\thl.env(\"A\", \"1\")", "\t-- hl.env(\"A\", \"1\")"},
		{"indent kept when uncommenting", "  -- hl.env(\"A\", \"1\")\n", 0, "  hl.env(\"A\", \"1\")\n", "  -- hl.env(\"A\", \"1\")", "  hl.env(\"A\", \"1\")"},
		{"alignment spaces kept", "hl.env(\"A\",    \"1\") -- note\n", 0, "-- hl.env(\"A\",    \"1\") -- note\n", `hl.env("A",    "1") -- note`, `-- hl.env("A",    "1") -- note`},
		{"no trailing newline", "hl.env(\"A\", \"1\")", 0, "-- hl.env(\"A\", \"1\")", `hl.env("A", "1")`, `-- hl.env("A", "1")`},
		{"CRLF stays on its line", "x\r\nhl.env(\"A\", \"1\")\r\ny\r\n", 1, "x\r\n-- hl.env(\"A\", \"1\")\r\ny\r\n", `hl.env("A", "1")`, `-- hl.env("A", "1")`},
		{"only the target line of a file changes", "local x = 1\n\nhl.env(\"A\", \"1\")\n-- hl.env(\"B\", \"2\")\nhl.config({})\n", 3,
			"local x = 1\n\nhl.env(\"A\", \"1\")\nhl.env(\"B\", \"2\")\nhl.config({})\n", `-- hl.env("B", "2")`, `hl.env("B", "2")`},
		{"duplicate names change only the target", "hl.env(\"A\", \"1\")\nhl.env(\"A\", \"1\")\nhl.env(\"A\", \"1\")\n", 1,
			"hl.env(\"A\", \"1\")\n-- hl.env(\"A\", \"1\")\nhl.env(\"A\", \"1\")\n", `hl.env("A", "1")`, `-- hl.env("A", "1")`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, change, err := envvars.ToggleLine([]byte(tt.content), varAt(t, tt.content, tt.line))
			if err != nil {
				t.Fatalf("ToggleLine() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("ToggleLine() content = %q, want %q", got, tt.want)
			}
			if before, after := strings.TrimRight(change.Before, "\r"), strings.TrimRight(change.After, "\r"); before != tt.wantBefore || after != tt.wantAfter {
				t.Errorf("change = {%q, %q}, want {%q, %q}", before, after, tt.wantBefore, tt.wantAfter)
			}
		})
	}
}

func TestToggleLine_TwiceRestoresTheOriginalBytes(t *testing.T) {
	original := "-- header\r\n  hl.env(\"A\",   \"1\") -- note\r\nhl.env(\"B\", \"2\")"
	v := varAt(t, original, 1)

	off, _, err := envvars.ToggleLine([]byte(original), v)
	if err != nil {
		t.Fatal(err)
	}
	back, _, err := envvars.ToggleLine(off, varAt(t, string(off), 1))
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != original {
		t.Errorf("toggle twice = %q, want %q", back, original)
	}
}

func TestToggleLine_ReturnsErrLineChangedWhenTheLineIsNoLongerTheRowShown(t *testing.T) {
	const content = "hl.env(\"A\", \"1\")\n-- hl.env(\"B\", \"2\")\n"
	on := envvars.Var{Name: "A", Value: "1", Line: 0}
	tests := []struct {
		name string
		v    envvars.Var
	}{
		{"name differs", envvars.Var{Name: "Z", Value: "1", Line: 0}},
		{"value differs", envvars.Var{Name: "A", Value: "2", Line: 0}},
		{"state differs", envvars.Var{Name: "A", Value: "1", Line: 0, Off: true}},
		{"line moved", envvars.Var{Name: "A", Value: "1", Line: 1}},
		{"line past the end", envvars.Var{Name: "A", Value: "1", Line: 9}},
		{"line is not an env call", envvars.Var{Name: "A", Value: "1", Line: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := envvars.ToggleLine([]byte(content), tt.v)
			if !errors.Is(err, envvars.ErrLineChanged) {
				t.Errorf("ToggleLine() error = %v, want ErrLineChanged", err)
			}
			if got != nil {
				t.Errorf("ToggleLine() content = %q, want nil on error", got)
			}
		})
	}
	if _, _, err := envvars.ToggleLine([]byte(content), on); err != nil {
		t.Errorf("sanity: matching row error = %v", err)
	}
}
