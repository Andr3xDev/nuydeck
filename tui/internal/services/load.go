package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type Source string

const (
	SourcePackage Source = "package"
	SourceCustom  Source = "custom"
)

const serviceTable = "service"

// Entry is one managed unit; the catalog of entries is the closed set of units the TUI may act on.
type Entry struct {
	Unit        string
	Scope       plan.Scope
	Source      Source
	Critical    bool
	Description string
}

type serviceDoc struct {
	Version int          `toml:"version"`
	Service []serviceRow `toml:"service"`
}

type serviceRow struct {
	Unit        string `toml:"unit"`
	Scope       string `toml:"scope"`
	Source      string `toml:"source"`
	Critical    bool   `toml:"critical"`
	Description string `toml:"description"`
}

// Load reads and validates the services catalog; errors carry the file name and entry line.
func Load(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	var doc serviceDoc
	if err := manifest.DecodeStrict(name, data, &doc); err != nil {
		return nil, err
	}
	entries := make([]Entry, len(doc.Service))
	for i, r := range doc.Service {
		entries[i] = Entry{Unit: r.Unit, Scope: plan.Scope(r.Scope), Source: Source(r.Source), Critical: r.Critical, Description: r.Description}
	}
	if err := validate(entries); err != nil {
		// EntryFailure flattens the error to text; re-wrap so callers can still errors.As the *EntryError.
		var entryErr *manifest.EntryError
		errors.As(err, &entryErr)
		return nil, &manifest.EntryError{Index: entryErr.Index, Prior: entryErr.Prior, Err: manifest.EntryFailure(name, data, serviceTable, err)}
	}
	return entries, nil
}

func validate(entries []Entry) error {
	type key struct {
		unit  string
		scope plan.Scope
	}
	seen := make(map[key]int, len(entries))
	for i, e := range entries {
		fail := func(prior int, err error) error { return &manifest.EntryError{Index: i, Prior: prior, Err: err} }
		// Reuses the step's own unit and scope validation so the catalog and the executor cannot drift.
		if _, err := (plan.ServiceActionStep{Unit: e.Unit, Scope: e.Scope, Action: plan.ActionStart}).Command(); err != nil {
			return fail(-1, err)
		}
		if e.Source != SourcePackage && e.Source != SourceCustom {
			return fail(-1, fmt.Errorf("unknown source %q", e.Source))
		}
		k := key{e.Unit, e.Scope}
		if prior, dup := seen[k]; dup {
			return fail(prior, fmt.Errorf("duplicate service %q (%s)", e.Unit, e.Scope))
		}
		seen[k] = i
	}
	return nil
}
