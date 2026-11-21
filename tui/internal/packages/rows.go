package packages

import (
	"cmp"
	"slices"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

// Status is how a package row compares to the machine.
type Status string

const (
	StatusInstalled Status = "installed"
	StatusMissing   Status = "missing"
	StatusDrift     Status = "drift"
)

// Row is one line of the packages table; drift rows are not in the catalog and have no group or source.
type Row struct {
	Name   string
	Group  string
	Source manifest.Source
	Status Status
}

// Rows lists the catalog by group then name as installed or missing, then the explicit packages the catalog does not know as drift.
func Rows(installed map[string]bool, explicit []string, catalog []manifest.Package, drivers []manifest.Driver, paruPrerequisites, ignored []string) []Row {
	sorted := slices.Clone(catalog)
	slices.SortFunc(sorted, func(a, b manifest.Package) int {
		return cmp.Or(cmp.Compare(a.Group, b.Group), cmp.Compare(a.Name, b.Name))
	})
	rows := make([]Row, 0, len(sorted))
	for _, p := range sorted {
		status := StatusMissing
		if installed[p.Name] {
			status = StatusInstalled
		}
		rows = append(rows, Row{Name: p.Name, Group: p.Group, Source: p.Source, Status: status})
	}
	for _, name := range Unregistered(explicit, catalog, drivers, paruPrerequisites, ignored) {
		rows = append(rows, Row{Name: name, Status: StatusDrift})
	}
	return rows
}
