package ui

// sectionSpec describes one sidebar section; the registry order is the sidebar order.
type sectionSpec struct {
	title   string
	enabled func(Deps) bool
	build   func(*shell) screen
}

func hasManifest(d Deps) bool { return d.ManifestError == nil }

// buildOrLoadError shows the error screen, naming fix, instead of a section whose manifest failed to load.
func buildOrLoadError(loadErr func(Deps) error, fix string, build func(*shell) screen) func(*shell) screen {
	return func(svc *shell) screen {
		if err := loadErr(svc.Deps); err != nil {
			return newErrorScreen(svc, err, fix)
		}
		return build(svc)
	}
}

func catalogError(d Deps) error { return d.CatalogError }

// sectionSpecs is the whole sidebar. A manifest error leaves only the first entry.
var sectionSpecs = []sectionSpec{
	{"Manifest", func(d Deps) bool { return d.ManifestError != nil }, func(svc *shell) screen {
		return newErrorScreen(svc, svc.ManifestError, "")
	}},
	{"Dashboard", hasManifest, func(svc *shell) screen { return newDashboard(svc) }},
	{"Sync", hasManifest, func(svc *shell) screen { return newSync(svc) }},
	{"Backups", hasManifest, func(svc *shell) screen { return newBackups(svc) }},
	{"Install", hasManifest, buildOrLoadError(catalogError, catalogFiles, func(svc *shell) screen { return newInstall(svc) })},
	{"Packages", hasManifest, buildOrLoadError(catalogError, catalogFiles, func(svc *shell) screen { return newPackages(svc) })},
	{"Services", func(d Deps) bool { return hasManifest(d) && (d.Services.Err != nil || d.Services.State != nil) },
		buildOrLoadError(func(d Deps) error { return d.Services.Err }, servicesFile, func(svc *shell) screen { return newServices(svc) })},
	{"Env", func(d Deps) bool { return hasManifest(d) && d.Env.Edit != nil && d.ReadSide != nil },
		func(svc *shell) screen { return newEnv(svc) }},
	{"References", func(d Deps) bool {
		return hasManifest(d) && (d.References.Err != nil || (d.References.Status != nil && d.References.Open != nil))
	}, buildOrLoadError(func(d Deps) error { return d.References.Err }, referencesFile, func(svc *shell) screen { return newReferences(svc) })},
}
