package app

import (
	"context"
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Packages manages explicit packages that the catalog does not know.
type Packages struct {
	Probe       SystemProbe
	Inspector   Inspector
	Runner      plan.Runner
	Catalog     []manifest.Package
	Drivers     []manifest.Driver
	CatalogPath string
}

// ListUnregistered returns the explicitly installed packages that are neither registered nor ignored.
func (p *Packages) ListUnregistered(ctx context.Context) ([]string, error) {
	explicit, err := p.Probe.ExplicitPackages(ctx)
	if err != nil {
		return nil, err
	}
	state, _ := p.Inspector.LoadState(ctx)
	return domain.Unregistered(explicit, p.Catalog, p.Drivers, manifest.ParuPrerequisites, state.IgnoredPackages), nil
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
	state, _ := p.Inspector.LoadState(ctx)
	if !slices.Contains(state.IgnoredPackages, name) {
		state.IgnoredPackages = append(slices.Clone(state.IgnoredPackages), name)
	}
	return p.Runner.Run(ctx, plan.SaveStateStep{State: state}, nil)
}
