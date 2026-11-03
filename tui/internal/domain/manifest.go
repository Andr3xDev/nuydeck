package domain

// Root names a managed destination root; the repo folder has the same name.
type Root string

const (
	RootConfig Root = "config"
	RootHome   Root = "home"
	RootFonts  Root = "fonts"
	RootThemes Root = "themes"
)

// Manifest mirrors manifest/paths.toml.
type Manifest struct {
	Version  int             `toml:"version"`
	Roots    map[Root]string `toml:"roots"`
	Entries  []Entry         `toml:"entry"`
	Excludes []Exclude       `toml:"exclude"`
}

// Entry is one managed file or directory, relative to its root.
type Entry struct {
	Root Root   `toml:"root"`
	Path string `toml:"path"`
}

// Exclude is a root-relative rsync glob.
type Exclude struct {
	Root    Root   `toml:"root"`
	Pattern string `toml:"pattern"`
}

// ResolvedEntry is an Entry with absolute repo and destination paths.
type ResolvedEntry struct {
	Entry
	RepoPath string
	DestPath string
	Filter   Filter
}

// Filter holds rsync exclude rules already anchored to one entry.
type Filter struct {
	Excludes []string
}
