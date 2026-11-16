package ui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

type fakePackageManager struct {
	names []string
	adds  []manifest.Package
}

func (f *fakePackageManager) ListUnregistered(context.Context) ([]string, error) { return f.names, nil }

func (f *fakePackageManager) AddPackage(_ context.Context, pkg manifest.Package) (domain.StepResult, error) {
	f.adds = append(f.adds, pkg)
	return domain.StepResult{Status: domain.StepOK}, nil
}

func (f *fakePackageManager) IgnorePackage(context.Context, string) domain.StepResult {
	return domain.StepResult{Status: domain.StepOK}
}

func TestPackagesAddFormRejectsEmptySource(t *testing.T) {
	f := &fakePackageManager{names: []string{"htop"}}
	p := NewRoot("", false, Deps{Packages: f}).sections[packagesSection].screen.(*Packages)
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
