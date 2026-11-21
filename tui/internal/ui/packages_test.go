package ui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/packages"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type fakePackagesDeps struct {
	rows    []packages.Row
	listErr error
	listed  int
	adds    []manifest.Package
	ignored []string
	ran     []plan.Plan
}

func (f *fakePackagesDeps) deps() Deps {
	return Deps{
		RunPlan: f.RunPlan,
		Packages: PackagesDeps{
			List:          f.List,
			AddPackage:    f.AddPackage,
			IgnorePackage: f.IgnorePackage,
		},
	}
}

func (f *fakePackagesDeps) List(context.Context) ([]packages.Row, error) {
	f.listed++
	return f.rows, f.listErr
}

func (f *fakePackagesDeps) AddPackage(_ context.Context, pkg manifest.Package) (plan.StepResult, error) {
	f.adds = append(f.adds, pkg)
	return plan.StepResult{Status: plan.StepOK}, nil
}

func (f *fakePackagesDeps) IgnorePackage(_ context.Context, name string) plan.StepResult {
	f.ignored = append(f.ignored, name)
	return plan.StepResult{Status: plan.StepOK}
}

func (f *fakePackagesDeps) RunPlan(_ context.Context, p plan.Plan, _ func(plan.Event)) error {
	f.ran = append(f.ran, p)
	return nil
}

func sampleRows() []packages.Row {
	return []packages.Row{
		{Name: "git", Group: "dev", Source: manifest.SourcePacman, Status: packages.StatusInstalled},
		{Name: "htop", Group: "tools", Source: manifest.SourcePacman, Status: packages.StatusMissing},
		{Name: "yay-bin", Group: "tools", Source: manifest.SourceAUR, Status: packages.StatusMissing},
		{Name: "strace", Status: packages.StatusDrift},
	}
}

func loadedPackages(f *fakePackagesDeps) *Packages {
	p := newPackages(testServices(f.deps()))
	p.Update(p.load()())
	return p
}

// pacmanArgv is the argv of the one command a built plan runs.
func pacmanArgv(t *testing.T, p plan.Plan) []string {
	t.Helper()
	for _, s := range p.Steps {
		if c, ok := s.(plan.CommandStep); ok && c.Sudo {
			return c.Argv
		}
	}
	t.Fatalf("plan has no sudo command: %+v", p.Steps)
	return nil
}

func TestPackagesRenderAsATableWithCounts(t *testing.T) {
	p := loadedPackages(&fakePackagesDeps{rows: sampleRows()})

	view := viewOf(p)

	for _, want := range []string{"Package", "Group", "Status", "git", "htop", "strace", "4 packages: 1 installed, 2 missing, 1 drift"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestPackagesQueryFailureShowsTheErrorAndNoTable(t *testing.T) {
	p := loadedPackages(&fakePackagesDeps{listErr: errors.New("pacman: boom")})

	view := viewOf(p)

	if !strings.Contains(view, "pacman: boom") || strings.Contains(view, "Group") {
		t.Errorf("want the error and no table:\n%s", view)
	}
}

func TestPackagesFilterCyclesAndClearsMarks(t *testing.T) {
	p := loadedPackages(&fakePackagesDeps{rows: sampleRows()})
	send(p, keyPress(tea.KeySpace))

	send(p, keyPress('f'))
	if view := viewOf(p); strings.Contains(view, "git") || !strings.Contains(view, "htop") || strings.Contains(view, "strace") {
		t.Errorf("missing filter must show only missing rows:\n%s", view)
	}
	if len(p.targets()) != 1 || p.marks["git"] {
		t.Errorf("filter must clear marks: marks=%v", p.marks)
	}
	send(p, keyPress('f'))
	if view := viewOf(p); !strings.Contains(view, "strace") || strings.Contains(view, "htop") {
		t.Errorf("drift filter must show only drift rows:\n%s", view)
	}
	send(p, keyPress('f'))
	if view := viewOf(p); !strings.Contains(view, "git") || !strings.Contains(view, "strace") {
		t.Errorf("third f must show every row again:\n%s", view)
	}
}

func TestPackagesMarkAndMarkAllCoverVisibleRows(t *testing.T) {
	p := loadedPackages(&fakePackagesDeps{rows: sampleRows()})

	send(p, keyPress('f'), keyPress('a'))
	if !p.marks["htop"] || !p.marks["yay-bin"] || p.marks["git"] || p.marks["strace"] {
		t.Fatalf("a must mark only the visible rows: %v", p.marks)
	}
	send(p, keyPress('a'))
	if slices.Contains([]bool{p.marks["htop"], p.marks["yay-bin"]}, true) {
		t.Errorf("a again must clear the marks: %v", p.marks)
	}
}

func TestPackagesJumpMovesTheCursorAndClamps(t *testing.T) {
	p := loadedPackages(&fakePackagesDeps{rows: sampleRows()})

	send(p, keyPress('J'))

	if p.cursor != len(sampleRows())-1 {
		t.Errorf("cursor = %d, want the last row", p.cursor)
	}
}

func TestPackagesInstallRunsMarkedMissingRowsAndDropsAUR(t *testing.T) {
	f := &fakePackagesDeps{rows: sampleRows()}
	p := loadedPackages(f)

	send(p, keyPress('f'), keyPress('a'))
	runs := send(p, keyPress('i'), keyEnter, keyPress('y'))
	for _, r := range runs {
		runWork(r)
	}

	if len(f.ran) != 1 {
		t.Fatalf("ran %d plans, want 1", len(f.ran))
	}
	if argv := pacmanArgv(t, f.ran[0]); !slices.Equal(argv, []string{"pacman", "-S", "--needed", "--noconfirm", "htop"}) {
		t.Errorf("argv = %v, want only the pacman missing package", argv)
	}
	if !strings.Contains(p.notice, "yay-bin") {
		t.Errorf("notice = %q, want the AUR package named", p.notice)
	}
}

func TestPackagesInstallOnNonMissingRowOnlyNotices(t *testing.T) {
	f := &fakePackagesDeps{rows: sampleRows()}
	p := loadedPackages(f)

	runs := send(p, keyPress('i'))

	if len(runs) != 0 || p.flow.active() || !strings.Contains(p.notice, "no marked package is missing") {
		t.Errorf("runs=%d active=%v notice=%q, want a notice and no flow", len(runs), p.flow.active(), p.notice)
	}
}

func TestPackagesUninstallNeedsBothConfirms(t *testing.T) {
	f := &fakePackagesDeps{rows: sampleRows()}
	p := loadedPackages(f)

	if runs := send(p, keyPress('u'), keyEnter, keyPress('y')); len(runs) != 0 || len(f.ran) != 0 {
		t.Fatalf("first confirm alone must not run: runs=%d ran=%d", len(runs), len(f.ran))
	}
	for _, r := range send(p, keyPress('y')) {
		runWork(r)
	}

	if len(f.ran) != 1 || !slices.Equal(pacmanArgv(t, f.ran[0]), []string{"pacman", "-Rns", "--noconfirm", "git"}) {
		t.Errorf("ran = %+v, want one pacman -Rns git", f.ran)
	}
}

func TestPackagesUninstallDeclinedAtTheExtraConfirmRunsNothing(t *testing.T) {
	f := &fakePackagesDeps{rows: sampleRows()}
	p := loadedPackages(f)

	runs := send(p, keyPress('u'), keyEnter, keyPress('y'), keyPress('n'))

	if len(runs) != 0 || len(f.ran) != 0 || p.flow.active() {
		t.Errorf("declined extra confirm must not run: runs=%d ran=%d active=%v", len(runs), len(f.ran), p.flow.active())
	}
}

func TestPackagesReloadAfterItsOwnRunCloses(t *testing.T) {
	f := &fakePackagesDeps{rows: sampleRows()}
	p := loadedPackages(f)
	send(p, keyPress('f'), keyPress('i'), keyEnter, keyPress('y'))
	before := f.listed

	_, cmd := p.Update(runClosed{})
	cmd()

	if f.listed != before+1 {
		t.Errorf("list calls = %d, want a reload after the run", f.listed)
	}
}

func TestPackagesAddAndIgnoreActOnlyOnDriftRows(t *testing.T) {
	f := &fakePackagesDeps{rows: sampleRows()}
	p := loadedPackages(f)

	send(p, keyPress('m'), keyPress('g'))
	if p.form != nil || len(f.ignored) != 0 || p.notice == "" {
		t.Fatalf("non-drift row: form=%v ignored=%v notice=%q", p.form != nil, f.ignored, p.notice)
	}

	send(p, keyPress('f'), keyPress('f'), keyPress('g'))
	if !slices.Equal(f.ignored, []string{"strace"}) {
		t.Errorf("ignored = %v, want the drift row", f.ignored)
	}
	send(p, keyPress('m'))
	if p.form == nil || p.form.name != "strace" {
		t.Error("m on a drift row must open the add form")
	}
}

func TestPackagesAddFormRejectsEmptySource(t *testing.T) {
	f := &fakePackagesDeps{rows: []packages.Row{{Name: "htop", Status: packages.StatusDrift}}}
	p := loadedPackages(f)

	send(p, keyPress('m'))
	if p.form == nil {
		t.Fatal("m did not open the add form")
	}
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil || len(f.adds) != 0 {
		t.Errorf("empty source issued a write: cmd=%v adds=%v", cmd != nil, f.adds)
	}
	if p.form == nil || p.form.err == "" {
		t.Error("form closed or shows no error for the empty source")
	}
}
