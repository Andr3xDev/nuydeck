package ui

// sectionSpec describes one sidebar section; the registry order is the sidebar order.
type sectionSpec struct {
	title   string
	enabled func(Deps) bool
	build   func(*services) screen
}

func hasManifest(d Deps) bool { return d.ManifestError == nil }

// buildOrCatalogError shows the error screen instead of a section that needs the package catalog.
func buildOrCatalogError(build func(*services) screen) func(*services) screen {
	return func(svc *services) screen {
		if svc.CatalogError != nil {
			return newErrorScreen(svc, svc.CatalogError, catalogFiles)
		}
		return build(svc)
	}
}

// sectionSpecs is the whole sidebar. A manifest error leaves only the first entry.
var sectionSpecs = []sectionSpec{
	{"Manifest", func(d Deps) bool { return d.ManifestError != nil }, func(svc *services) screen {
		return newErrorScreen(svc, svc.ManifestError, "")
	}},
	{"Dashboard", hasManifest, func(svc *services) screen { return newDashboard(svc) }},
	{"Sync", hasManifest, func(svc *services) screen { return newSync(svc) }},
	{"Backups", hasManifest, func(svc *services) screen { return newBackups(svc) }},
	{"Install", hasManifest, buildOrCatalogError(func(svc *services) screen { return newInstall(svc) })},
	{"Packages", hasManifest, buildOrCatalogError(func(svc *services) screen { return newPackages(svc) })},
}
