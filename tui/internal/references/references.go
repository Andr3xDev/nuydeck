package references

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

const referenceTable = "reference"

// Reference is one external link with optional presence detection.
type Reference struct {
	Name         string
	Description  string
	URL          string
	DetectBinary string
	DetectPath   string
}

type referenceDoc struct {
	Version   int            `toml:"version"`
	Reference []referenceRow `toml:"reference"`
}

type referenceRow struct {
	Name         string `toml:"name"`
	Description  string `toml:"description"`
	URL          string `toml:"url"`
	DetectBinary string `toml:"detect_binary"`
	DetectPath   string `toml:"detect_path"`
}

// Load reads and validates the references manifest; errors carry the file name and entry line.
func Load(path string) ([]Reference, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	var doc referenceDoc
	if err := manifest.DecodeStrict(name, data, &doc); err != nil {
		return nil, err
	}
	refs := make([]Reference, len(doc.Reference))
	for i, r := range doc.Reference {
		refs[i] = Reference(r)
	}
	if err := validate(refs); err != nil {
		return nil, manifest.EntryFailure(name, data, referenceTable, err)
	}
	return refs, nil
}

func validate(refs []Reference) error {
	seen := make(map[string]int, len(refs))
	for i, r := range refs {
		fail := func(prior int, err error) error { return &manifest.EntryError{Index: i, Prior: prior, Err: err} }
		if r.Name == "" {
			return fail(-1, errors.New("empty name"))
		}
		if r.Description == "" {
			return fail(-1, fmt.Errorf("reference %q: missing description", r.Name))
		}
		if err := validateURL(r.URL); err != nil {
			return fail(-1, fmt.Errorf("reference %q: %w", r.Name, err))
		}
		if prior, dup := seen[r.Name]; dup {
			return fail(prior, fmt.Errorf("duplicate reference %q", r.Name))
		}
		seen[r.Name] = i
	}
	return nil
}

func validateURL(raw string) error {
	if strings.IndexFunc(raw, func(c rune) bool { return unicode.IsSpace(c) || unicode.IsControl(c) }) >= 0 {
		return fmt.Errorf("url %q has whitespace or control characters", raw)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("url %q must be an https link with a host", raw)
	}
	return nil
}
