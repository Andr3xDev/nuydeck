package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const catalogPath = "/repo/manifest/packages.toml"

func newPackages(probe *fakeProbe, in *fakeInspector, r Runner) *Packages {
	return &Packages{
		Probe:       probe,
		Inspector:   in,
		Runner:      r,
		CatalogPath: catalogPath,
		Catalog:     []domain.Package{{Name: "hyprland", Source: domain.SourcePacman, Profiles: []string{"minimal"}}},
		Drivers:     []domain.Driver{{ID: "nvidia", VendorID: "10de", Packages: []string{"nvidia-utils"}}},
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
			in := &fakeInspector{state: domain.State{IgnoredPackages: tc.ignored}}
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

func TestPackages_AddPackageRunsAppendStepAndReturnsTheRunnerResult(t *testing.T) {
	wouldApply := domain.StepResult{Status: domain.StepWouldApply, Detail: "would append htop"}
	r := &fakeRunner{result: func(domain.Step) domain.StepResult { return wouldApply }}
	pkg := domain.Package{Name: "htop", Source: domain.SourcePacman, Profiles: []string{"full"}, Group: "tools"}

	res, err := newPackages(&fakeProbe{}, &fakeInspector{}, r).AddPackage(context.Background(), pkg)

	if err != nil {
		t.Fatal(err)
	}
	steps := stepsOfType[domain.AppendPackageStep](r.steps)
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
		status     domain.StepStatus
		wantInList bool
	}{
		{"written", domain.StepOK, true},
		{"dry-run", domain.StepWouldApply, false},
		{"failed", domain.StepFailed, false},
	}
	pkg := domain.Package{Name: "htop", Source: domain.SourcePacman, Profiles: []string{"full"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{result: func(domain.Step) domain.StepResult { return domain.StepResult{Status: tc.status} }}
			p := newPackages(&fakeProbe{}, &fakeInspector{}, r)

			if _, err := p.AddPackage(context.Background(), pkg); err != nil {
				t.Fatal(err)
			}

			if got := slices.ContainsFunc(p.Catalog, func(c domain.Package) bool { return c.Name == "htop" }); got != tc.wantInList {
				t.Errorf("htop in catalog = %v, want %v", got, tc.wantInList)
			}
			if tc.wantInList {
				if _, err := p.AddPackage(context.Background(), pkg); !errors.Is(err, domain.ErrDuplicatePackage) {
					t.Errorf("second add err = %v, want ErrDuplicatePackage", err)
				}
			}
		})
	}
}

func TestPackages_AddPackageWithoutSourceIsRejectedBeforeAnyWrite(t *testing.T) {
	r := &fakeRunner{}

	_, err := newPackages(&fakeProbe{}, &fakeInspector{}, r).AddPackage(context.Background(), domain.Package{Name: "htop", Profiles: []string{"full"}})

	if !errors.Is(err, domain.ErrUnknownSource) {
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
			state := domain.State{Version: 1, RepoPath: "/repo", Ignores: []string{"config:hypr/a.conf"}, IgnoredPackages: tc.existing}
			r := &fakeRunner{}

			newPackages(&fakeProbe{}, &fakeInspector{state: state}, r).IgnorePackage(context.Background(), "htop")

			saves := stepsOfType[domain.SaveStateStep](r.steps)
			if len(saves) != 1 {
				t.Fatalf("steps = %v, want one SaveStateStep", r.steps)
			}
			got := saves[0].State
			if !slices.Equal(got.IgnoredPackages, tc.want) {
				t.Errorf("IgnoredPackages = %v, want %v", got.IgnoredPackages, tc.want)
			}
			if !slices.Equal(got.Ignores, state.Ignores) || got.RepoPath != "/repo" || got.Version != 1 {
				t.Errorf("sync state changed: %+v", got)
			}
		})
	}
}
