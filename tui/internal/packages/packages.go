package packages

import (
	"context"
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Probe is the read-only machine query the packages screen needs.
type Probe interface {
	ExplicitPackages(ctx context.Context) ([]string, error)
	InstalledPackages(ctx context.Context) (map[string]bool, error)
}

// Packages manages explicit packages that the catalog does not know.
type Packages struct {
	Probe Probe
	// LoadState never fails; a non-empty warning means defaults were used.
	LoadState   func(ctx context.Context) (state.State, string)
	Runner      plan.Runner
	Catalog     []manifest.Package
	Drivers     []manifest.Driver
	CatalogPath string
}

// List returns the catalog rows and the drift rows; any failed query returns its error and no rows.
func (p *Packages) List(ctx context.Context) ([]Row, error) {
	installed, err := p.Probe.InstalledPackages(ctx)
	if err != nil {
		return nil, err
	}
	explicit, err := p.Probe.ExplicitPackages(ctx)
	if err != nil {
		return nil, err
	}
	saved, _ := p.LoadState(ctx)
	return Rows(installed, explicit, p.Catalog, p.Drivers, manifest.ParuPrerequisites, saved.IgnoredPackages), nil
}

// AddPackage appends pkg to the catalog file; an invalid or duplicate package is rejected before any write.
// The in-memory catalog follows only a real write, so a dry-run leaves it untouched.
func (p *Packages) AddPackage(ctx context.Context, pkg manifest.Package) (plan.StepResult, error) {
	if err := manifest.ValidatePackages(append(slices.Clone(p.Catalog), pkg)); err != nil {
		return plan.StepResult{}, err
	}
	res := p.Runner.Run(ctx, plan.AppendPackageStep{Path: p.CatalogPath, Entry: pkg}, nil)
	if res.Status == plan.StepOK {
		p.Catalog = append(p.Catalog, pkg)
	}
	return res, nil
}

// IgnorePackage adds name to the ignored packages in the state, keeping the rest of the state as saved.
func (p *Packages) IgnorePackage(ctx context.Context, name string) plan.StepResult {
	saved, _ := p.LoadState(ctx)
	if !slices.Contains(saved.IgnoredPackages, name) {
		saved.IgnoredPackages = append(slices.Clone(saved.IgnoredPackages), name)
	}
	return p.Runner.Run(ctx, plan.SaveStateStep{State: saved}, nil)
}
