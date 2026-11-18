package ui

// sectionSpec describes one sidebar section; the registry order is the sidebar order.
type sectionSpec struct {
	title   string
	enabled func(Deps) bool
	build   func(*shell) screen
}

func hasManifest(d Deps) bool { return d.ManifestError == nil }

// buildOrCatalogError shows the error screen instead of a section that needs the package catalog.
func buildOrCatalogError(build func(*shell) screen) func(*shell) screen {
	return func(svc *shell) screen {
		if svc.CatalogError != nil {
			return newErrorScreen(svc, svc.CatalogError, catalogFiles)
		}
		return build(svc)
	}
}

// sectionSpecs is the whole sidebar. A manifest error leaves only the first entry.
var sectionSpecs = []sectionSpec{
	{"Manifest", func(d Deps) bool { return d.ManifestError != nil }, func(svc *shell) screen {
		return newErrorScreen(svc, svc.ManifestError, "")
	}},
	{"Dashboard", hasManifest, func(svc *shell) screen { return newDashboard(svc) }},
	{"Sync", hasManifest, func(svc *shell) screen { return newSync(svc) }},
	{"Backups", hasManifest, func(svc *shell) screen { return newBackups(svc) }},
	{"Install", hasManifest, buildOrCatalogError(func(svc *shell) screen { return newInstall(svc) })},
	{"Packages", hasManifest, buildOrCatalogError(func(svc *shell) screen { return newPackages(svc) })},
}
