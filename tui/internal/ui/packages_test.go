package ui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type fakePackagesDeps struct {
	names []string
	adds  []manifest.Package
}

func (f *fakePackagesDeps) deps() PackagesDeps {
	return PackagesDeps{ListUnregistered: f.ListUnregistered, AddPackage: f.AddPackage, IgnorePackage: f.IgnorePackage}
}

func (f *fakePackagesDeps) ListUnregistered(context.Context) ([]string, error) { return f.names, nil }

func (f *fakePackagesDeps) AddPackage(_ context.Context, pkg manifest.Package) (plan.StepResult, error) {
	f.adds = append(f.adds, pkg)
	return plan.StepResult{Status: plan.StepOK}, nil
}

func (f *fakePackagesDeps) IgnorePackage(context.Context, string) plan.StepResult {
	return plan.StepResult{Status: plan.StepOK}
}

func TestPackagesAddFormRejectsEmptySource(t *testing.T) {
	f := &fakePackagesDeps{names: []string{"htop"}}
	p, _, _ := sectionOf[*Packages](NewRoot("", false, Deps{Packages: f.deps()}))
	p.Update(unregisteredMsg{names: f.names})

	p.Update(keyPress('a'))
	if p.form == nil {
		t.Fatal("a did not open the add form")
	}
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil || len(f.adds) != 0 {
		t.Errorf("empty source issued a write: cmd=%v adds=%v", cmd != nil, f.adds)
	}
	if p.form == nil || p.form.err == "" {
		t.Error("form closed or shows no error for the empty source")
	}
}
