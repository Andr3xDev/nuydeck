package services_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

const repoServices = "../../../manifest/services.toml"

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "services.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func lines(l ...string) string { return strings.Join(l, "\n") + "\n" }

func TestLoad_ValidEntriesWithOptionalFieldsAbsent(t *testing.T) {
	path := writeTemp(t, lines(
		`version = 1`,
		``,
		`[[service]]`,
		`unit = "NetworkManager.service"`,
		`scope = "system"`,
		`source = "package"`,
		`critical = true`,
		`description = "network manager"`,
		``,
		`[[service]]`,
		`unit = "clip.timer"`,
		`scope = "user"`,
		`source = "custom"`,
	))
	got, err := services.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []services.Entry{
		{Unit: "NetworkManager.service", Scope: plan.ScopeSystem, Source: services.SourcePackage, Critical: true, Description: "network manager"},
		{Unit: "clip.timer", Scope: plan.ScopeUser, Source: services.SourceCustom},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_ErrorsNameFileAndLine(t *testing.T) {
	entry := func(unit, scope, source string) []string {
		return []string{``, `[[service]]`, `unit = "` + unit + `"`, `scope = "` + scope + `"`, `source = "` + source + `"`}
	}
	build := func(parts ...[]string) string {
		all := []string{`version = 1`}
		for _, p := range parts {
			all = append(all, p...)
		}
		return lines(all...)
	}
	tests := []struct {
		name     string
		content  string
		wantLine int
		wantText []string
	}{
		{"syntax error", lines(`version = 1`, ``, `[[service]`, `unit = "a.service"`), 3, nil},
		{"unknown key", build(append(entry("a.service", "system", "package"), `colour = "red"`)), 7, []string{"colour"}},
		{"leading dash unit", build(entry("-a.service", "system", "package")), 3, []string{"-a.service"}},
		{"unit without suffix", build(entry("a", "system", "package")), 3, []string{"a"}},
		{"unsupported unit type", build(entry("a.socket", "system", "package")), 3, []string{"a.socket"}},
		{"unknown scope", build(entry("a.service", "global", "package")), 3, []string{"global"}},
		{"unknown source", build(entry("a.service", "system", "flatpak")), 3, []string{"flatpak"}},
		{
			"duplicate unit and scope names both lines",
			build(entry("a.service", "system", "package"), entry("a.service", "system", "custom")),
			8, []string{"a.service", "line 3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := services.Load(writeTemp(t, tt.content))
			if err == nil {
				t.Fatal("Load() error = nil, want error")
			}
			if prefix := "services.toml:" + strconv.Itoa(tt.wantLine) + ": "; !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("error = %q, want prefix %q", err, prefix)
			}
			for _, text := range tt.wantText {
				if !strings.Contains(err.Error(), text) {
					t.Errorf("error = %q, want it to contain %q", err, text)
				}
			}
		})
	}
}

func TestLoad_SameUnitInDifferentScopesIsNotDuplicate(t *testing.T) {
	path := writeTemp(t, lines(
		`version = 1`, ``,
		`[[service]]`, `unit = "a.service"`, `scope = "system"`, `source = "package"`, ``,
		`[[service]]`, `unit = "a.service"`, `scope = "user"`, `source = "custom"`,
	))
	if _, err := services.Load(path); err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}
}

func TestLoad_ValidationErrorIsEntryError(t *testing.T) {
	path := writeTemp(t, lines(`version = 1`, ``, `[[service]]`, `unit = "a"`, `scope = "system"`, `source = "package"`))
	_, err := services.Load(path)
	var entryErr *manifest.EntryError
	if !errors.As(err, &entryErr) {
		t.Errorf("error = %v, want it to wrap *manifest.EntryError", err)
	}
}

func TestLoad_MissingFileIsAnError(t *testing.T) {
	if _, err := services.Load(filepath.Join(t.TempDir(), "services.toml")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want os.ErrNotExist", err)
	}
}

func TestRepoServicesKeepContentRules(t *testing.T) {
	entries, err := services.Load(repoServices)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	find := func(unit string, scope plan.Scope) (services.Entry, bool) {
		for _, e := range entries {
			if e.Unit == unit && e.Scope == scope {
				return e, true
			}
		}
		return services.Entry{}, false
	}

	for _, unit := range []string{"ly@tty1.service", "NetworkManager.service"} {
		if e, ok := find(unit, plan.ScopeSystem); !ok || !e.Critical {
			t.Errorf("%s = %+v (found %v), want present and critical", unit, e, ok)
		}
	}
	for _, banned := range []string{"ly.service", "systemd-logind.service"} {
		for _, e := range entries {
			if e.Unit == banned {
				t.Errorf("services.toml lists %q", banned)
			}
		}
	}
	if e, ok := find("lucyna-clipboard-wipe.timer", plan.ScopeUser); !ok || e.Source != services.SourceCustom {
		t.Errorf("lucyna timer = %+v (found %v), want its own user custom entry", e, ok)
	}
}

func TestRepoServicesCoverCatalogUnits(t *testing.T) {
	entries, err := services.Load(repoServices)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	packages, err := manifest.LoadPackages("../../../manifest/packages.toml")
	if err != nil {
		t.Fatal(err)
	}
	drivers, err := manifest.LoadHardware("../../../manifest/hardware.toml")
	if err != nil {
		t.Fatal(err)
	}
	var units []string
	for _, p := range packages {
		if p.Service != "" {
			units = append(units, p.Service)
		}
	}
	for _, d := range drivers {
		units = append(units, d.Services...)
	}
	if len(units) == 0 {
		t.Fatal("catalog declares no service units")
	}
	for _, unit := range units {
		if !slices.ContainsFunc(entries, func(e services.Entry) bool { return e.Unit == unit && e.Scope == plan.ScopeSystem }) {
			t.Errorf("catalog unit %q missing from services.toml with scope system", unit)
		}
	}
}
