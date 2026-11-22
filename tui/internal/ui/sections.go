package ui

const (
	catalogFiles   = "manifest/packages.toml, hardware.toml or secrets.toml"
	servicesFile   = "manifest/services.toml"
	referencesFile = "manifest/references.toml"
)

// sectionSpec describes one sidebar section; the registry order is the sidebar order.
type sectionSpec struct {
	title   string
	enabled func(Deps) bool
	build   func(*shell) screen
	pinned  bool // drawn on the bottom rows of the sidebar; pinned specs must come last
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

// sectionSpecs is the whole sidebar. A manifest error leaves only the first entry and Config.
var sectionSpecs = []sectionSpec{
	{title: "Manifest", enabled: func(d Deps) bool { return d.ManifestError != nil }, build: func(svc *shell) screen {
		return newErrorScreen(svc, svc.ManifestError, "")
	}},
	{title: "Dashboard", enabled: hasManifest, build: func(svc *shell) screen { return newDashboard(svc) }},
	{title: "Sync", enabled: hasManifest, build: func(svc *shell) screen { return newSync(svc) }},
	{title: "Backups", enabled: hasManifest, build: func(svc *shell) screen { return newBackups(svc) }},
	{title: "Install", enabled: hasManifest, build: buildOrLoadError(catalogError, catalogFiles, func(svc *shell) screen { return newInstall(svc) })},
	{title: "Packages", enabled: hasManifest, build: buildOrLoadError(catalogError, catalogFiles, func(svc *shell) screen { return newPackages(svc) })},
	{title: "Services", enabled: func(d Deps) bool { return hasManifest(d) && (d.Services.Err != nil || d.Services.State != nil) },
		build: buildOrLoadError(func(d Deps) error { return d.Services.Err }, servicesFile, func(svc *shell) screen { return newServices(svc) })},
	{title: "Env", enabled: func(d Deps) bool { return hasManifest(d) && d.Env.Edit != nil && d.ReadSide != nil },
		build: func(svc *shell) screen { return newEnv(svc) }},
	{title: "References", enabled: func(d Deps) bool {
		return hasManifest(d) && (d.References.Err != nil || (d.References.Status != nil && d.References.Open != nil))
	}, build: buildOrLoadError(func(d Deps) error { return d.References.Err }, referencesFile, func(svc *shell) screen { return newReferences(svc) })},
	{title: "Config", enabled: func(Deps) bool { return true }, build: func(svc *shell) screen { return newConfig(svc) }, pinned: true},
}
