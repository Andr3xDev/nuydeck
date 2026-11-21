package envvars_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/envvars"
)

const repoEnvFile = "../../../config/hypr/configs/env_variables.lua"

func lines(l ...string) string { return strings.Join(l, "\n") + "\n" }

func TestParseHlEnv_ListsOnlyWellFormedCalls(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []envvars.Var
	}{
		{"plain call", lines(`hl.env("A", "1")`), []envvars.Var{{Name: "A", Value: "1"}}},
		{"extra spaces and indent", lines(`   hl.env(  "A"  ,   "1"  )`), []envvars.Var{{Name: "A", Value: "1"}}},
		{"empty value", lines(`hl.env("A", "")`), []envvars.Var{{Name: "A", Value: ""}}},
		{"value with separators", lines(`hl.env("GDK_BACKEND", "wayland,x11,*")`), []envvars.Var{{Name: "GDK_BACKEND", Value: "wayland,x11,*"}}},
		{"trailing comment", lines(`hl.env("A", "1") -- note`), []envvars.Var{{Name: "A", Value: "1"}}},
		{"commented call is an off row", lines(`-- hl.env("A", "1")`), []envvars.Var{{Name: "A", Value: "1", Off: true}}},
		{"commented call without space", lines(`--hl.env("A", "1")`), []envvars.Var{{Name: "A", Value: "1", Off: true}}},
		{"indented commented call", lines(`  -- hl.env("A", "1")`), []envvars.Var{{Name: "A", Value: "1", Off: true}}},
		{"prose mentioning hl.env", lines(`-- use hl.env("A", "1") to set it`, `-- see hl.env docs`), nil},
		{"multi-line call", lines(`hl.env(`, `  "A",`, `  "1")`), nil},
		{"non-literal argument", lines(`hl.env("A", value)`), nil},
		{"commented non-literal argument", lines(`-- hl.env("A", value)`), nil},
		{"other code", lines(`local x = 1`, `hl.config({})`, ``), nil},
		{"empty text", "", nil},
		{"order kept", lines(`hl.env("B", "2")`, `hl.env("A", "1")`), []envvars.Var{{Name: "B", Value: "2"}, {Name: "A", Value: "1", Line: 1}}},
		{"line numbers count every line", lines(`local x = 1`, ``, `hl.env("A", "1")`, `-- note`, `-- hl.env("B", "2")`),
			[]envvars.Var{{Name: "A", Value: "1", Line: 2}, {Name: "B", Value: "2", Line: 4, Off: true}}},
		{"duplicates keep their own rows", lines(`hl.env("A", "1")`, `-- hl.env("A", "1")`, `hl.env("A", "2")`),
			[]envvars.Var{{Name: "A", Value: "1"}, {Name: "A", Value: "1", Line: 1, Off: true}, {Name: "A", Value: "2", Line: 2}}},
		{"CRLF line endings", "hl.env(\"A\", \"1\")\r\n-- hl.env(\"B\", \"2\")\r\n",
			[]envvars.Var{{Name: "A", Value: "1"}, {Name: "B", Value: "2", Line: 1, Off: true}}},
		{"no trailing newline", `hl.env("A", "1")`, []envvars.Var{{Name: "A", Value: "1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envvars.ParseHlEnv(tt.text)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ParseHlEnv() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseHlEnv_RealRepoFileListsSeventeenVariables(t *testing.T) {
	data, err := os.ReadFile(repoEnvFile)
	if err != nil {
		t.Fatal(err)
	}
	got := envvars.ParseHlEnv(string(data))
	if len(got) != 17 {
		t.Fatalf("ParseHlEnv() listed %d variables, want 17: %+v", len(got), got)
	}
	want := envvars.Var{Name: "GDK_BACKEND", Value: "wayland,x11,*", Line: 32}
	if !slices.Contains(got, want) {
		t.Errorf("ParseHlEnv() missing %+v", want)
	}
	for _, v := range got {
		if v.Off {
			t.Errorf("%s listed off, the repo file has no commented variable", v.Name)
		}
	}
}
