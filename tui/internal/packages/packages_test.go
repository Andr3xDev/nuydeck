package packages

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const catalogPath = "/repo/manifest/packages.toml"

type fakeProbe struct {
	explicit     []string
	explicitErr  error
	installed    map[string]bool
	installedErr error
}

func (p *fakeProbe) ExplicitPackages(context.Context) ([]string, error) {
	return p.explicit, p.explicitErr
}

func (p *fakeProbe) InstalledPackages(context.Context) (map[string]bool, error) {
	return p.installed, p.installedErr
}

type fakeInspector struct{ state state.State }

func (f *fakeInspector) LoadState(context.Context) (state.State, string) { return f.state, "" }

type fakeRunner struct {
	mu     sync.Mutex
	steps  []plan.Step
	result func(plan.Step) plan.StepResult
}

func (r *fakeRunner) Run(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
	r.mu.Lock()
	r.steps = append(r.steps, s)
	r.mu.Unlock()
	if r.result != nil {
		return r.result(s)
	}
	return plan.StepResult{Status: plan.StepOK}
}

func stepsOfType[T plan.Step](steps []plan.Step) []T {
	var out []T
	for _, s := range steps {
		if v, ok := s.(T); ok {
			out = append(out, v)
		}
	}
	return out
}

func newPackages(probe *fakeProbe, in *fakeInspector, r plan.Runner) *Packages {
	return &Packages{
		Probe:       probe,
		LoadState:   in.LoadState,
		Runner:      r,
		CatalogPath: catalogPath,
		Catalog:     []manifest.Package{{Name: "hyprland", Source: manifest.SourcePacman, Profiles: []string{"minimal"}}},
		Drivers:     []manifest.Driver{{ID: "nvidia", VendorID: "10de", Packages: []string{"nvidia-utils"}}},
	}
}

func TestPackages_ListUnregisteredExcludesCatalogDriversPrerequisitesAndIgnored(t *testing.T) {
	cases := []struct {
		name     string
		explicit []string
		ignored  []string
		want     []string
	}{
		{
			name:     "unknown package is listed",
			explicit: []string{"hyprland", "nvidia-utils", "base-devel", "htop", "wget"},
			ignored:  []string{"wget"},
			want:     []string{"htop"},
		},
		{
			name:     "everything registered or ignored",
			explicit: []string{"hyprland", "git", "wget"},
			ignored:  []string{"wget"},
			want:     []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &fakeInspector{state: state.State{IgnoredPackages: tc.ignored}}
			got, err := newPackages(&fakeProbe{explicit: tc.explicit}, in, &fakeRunner{}).ListUnregistered(context.Background())
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("ListUnregistered = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestPackages_ListUnregisteredReturnsTheQueryErrorWithoutAList(t *testing.T) {
	queryErr := errors.New("pacman: exit status 1")
	probe := &fakeProbe{explicit: []string{"htop"}, explicitErr: queryErr}

	got, err := newPackages(probe, &fakeInspector{}, &fakeRunner{}).ListUnregistered(context.Background())

	if !errors.Is(err, queryErr) || got != nil {
		t.Fatalf("ListUnregistered = %v, %v; want nil and the query error", got, err)
	}
}

func TestPackages_ListMergesBothProbesIntoRows(t *testing.T) {
	probe := &fakeProbe{
		explicit:  []string{"hyprland", "htop", "wget"},
		installed: map[string]bool{"hyprland": true, "htop": true, "wget": true},
	}
	in := &fakeInspector{state: state.State{IgnoredPackages: []string{"wget"}}}

	got, err := newPackages(probe, in, &fakeRunner{}).List(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	status := map[string]Status{}
	for _, r := range got {
		status[r.Name] = r.Status
	}
	if len(got) != 2 || status["hyprland"] != "installed" || status["htop"] != "drift" {
		t.Errorf("List = %+v, want hyprland installed and htop drift only (wget ignored)", got)
	}
}

func TestPackages_ListReturnsTheQueryErrorAndNoRowsWhenAProbeFails(t *testing.T) {
	queryErr := errors.New("pacman: exit status 1")
	cases := map[string]*fakeProbe{
		"installed query fails": {explicit: []string{"htop"}, installedErr: queryErr},
		"explicit query fails":  {installed: map[string]bool{"hyprland": true}, explicitErr: queryErr},
	}
	for name, probe := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := newPackages(probe, &fakeInspector{}, &fakeRunner{}).List(context.Background())

			if !errors.Is(err, queryErr) || got != nil {
				t.Fatalf("List = %v, %v; want nil rows and the query error", got, err)
			}
		})
	}
}

func TestPackages_AddPackageRunsAppendStepAndReturnsTheRunnerResult(t *testing.T) {
	wouldApply := plan.StepResult{Status: plan.StepWouldApply, Detail: "would append htop"}
	r := &fakeRunner{result: func(plan.Step) plan.StepResult { return wouldApply }}
	pkg := manifest.Package{Name: "htop", Source: manifest.SourcePacman, Profiles: []string{"full"}, Group: "tools"}

	res, err := newPackages(&fakeProbe{}, &fakeInspector{}, r).AddPackage(context.Background(), pkg)

	if err != nil {
		t.Fatal(err)
	}
	steps := stepsOfType[plan.AppendPackageStep](r.steps)
	if len(steps) != 1 || steps[0].Path != catalogPath || !slices.Equal(steps[0].Entry.Profiles, pkg.Profiles) || steps[0].Entry.Name != "htop" || steps[0].Entry.Group != "tools" {
		t.Fatalf("steps = %+v, want one AppendPackageStep for htop at %s", r.steps, catalogPath)
	}
	if res.Status != wouldApply.Status || res.Detail != wouldApply.Detail {
		t.Errorf("result = %+v, want the runner result %+v", res, wouldApply)
	}
}

func TestPackages_AddPackageUpdatesTheCatalogOnlyAfterARealWrite(t *testing.T) {
	cases := []struct {
		name       string
		status     plan.StepStatus
		wantInList bool
	}{
		{"written", plan.StepOK, true},
		{"dry-run", plan.StepWouldApply, false},
		{"failed", plan.StepFailed, false},
	}
	pkg := manifest.Package{Name: "htop", Source: manifest.SourcePacman, Profiles: []string{"full"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{result: func(plan.Step) plan.StepResult { return plan.StepResult{Status: tc.status} }}
			p := newPackages(&fakeProbe{}, &fakeInspector{}, r)

			if _, err := p.AddPackage(context.Background(), pkg); err != nil {
				t.Fatal(err)
			}

			if got := slices.ContainsFunc(p.Catalog, func(c manifest.Package) bool { return c.Name == "htop" }); got != tc.wantInList {
				t.Errorf("htop in catalog = %v, want %v", got, tc.wantInList)
			}
			if tc.wantInList {
				if _, err := p.AddPackage(context.Background(), pkg); !errors.Is(err, manifest.ErrDuplicatePackage) {
					t.Errorf("second add err = %v, want ErrDuplicatePackage", err)
				}
			}
		})
	}
}

func TestPackages_AddPackageWithoutSourceIsRejectedBeforeAnyWrite(t *testing.T) {
	r := &fakeRunner{}

	_, err := newPackages(&fakeProbe{}, &fakeInspector{}, r).AddPackage(context.Background(), manifest.Package{Name: "htop", Profiles: []string{"full"}})

	if !errors.Is(err, manifest.ErrUnknownSource) {
		t.Errorf("err = %v, want ErrUnknownSource", err)
	}
	if len(r.steps) != 0 {
		t.Errorf("steps ran despite rejection: %v", r.steps)
	}
}

func TestPackages_IgnorePackageAppendsToIgnoredPackagesAndKeepsSyncState(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		want     []string
	}{
		{name: "first ignore", want: []string{"htop"}},
		{name: "keeps earlier ignores", existing: []string{"wget"}, want: []string{"wget", "htop"}},
		{name: "already ignored is not duplicated", existing: []string{"htop"}, want: []string{"htop"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved := state.State{Version: 1, RepoPath: "/repo", Ignores: []string{"config:hypr/a.conf"}, IgnoredPackages: tc.existing}
			r := &fakeRunner{}

			newPackages(&fakeProbe{}, &fakeInspector{state: saved}, r).IgnorePackage(context.Background(), "htop")

			saves := stepsOfType[plan.SaveStateStep](r.steps)
			if len(saves) != 1 {
				t.Fatalf("steps = %v, want one SaveStateStep", r.steps)
			}
			got := saves[0].State
			if !slices.Equal(got.IgnoredPackages, tc.want) {
				t.Errorf("IgnoredPackages = %v, want %v", got.IgnoredPackages, tc.want)
			}
			if !slices.Equal(got.Ignores, saved.Ignores) || got.RepoPath != "/repo" || got.Version != 1 {
				t.Errorf("sync state changed: %+v", got)
			}
		})
	}
}
