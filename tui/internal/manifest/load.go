package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// LoadManifest reads and validates the manifest; home expands the leading ~ of root destinations.
// Errors are prefixed with the file's base name and the offending line when known.
func LoadManifest(path, home string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	name := filepath.Base(path)
	var m Manifest
	if err := decodeStrict(name, data, &m); err != nil {
		return Manifest{}, err
	}

	for root, dest := range m.Roots {
		m.Roots[root] = expandHome(dest, home)
	}
	if err := m.Validate(home); err != nil {
		return Manifest{}, entryFailure(name, data, "entry", err)
	}
	return m, nil
}

// failAt prefixes msg with the file name and, when known, the 1-based line.
func failAt(name string, line int, msg string) error {
	if line == 0 {
		return fmt.Errorf("%s: %s", name, msg)
	}
	return fmt.Errorf("%s:%d: %s", name, line, msg)
}

// decodeStrict decodes data into out and rejects unknown keys, reporting file and line.
func decodeStrict(name string, data []byte, out any) error {
	dec := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	err := dec.Decode(out)
	if err == nil {
		return nil
	}
	var strict *toml.StrictMissingError
	var decodeErr *toml.DecodeError
	switch {
	case errors.As(err, &strict) && len(strict.Errors) > 0:
		first := strict.Errors[0]
		line, _ := first.Position()
		return failAt(name, line, fmt.Sprintf("unknown key %q", strings.Join(first.Key(), ".")))
	case errors.As(err, &decodeErr):
		line, _ := decodeErr.Position()
		return failAt(name, line, decodeErr.Error())
	}
	return failAt(name, 0, err.Error())
}

// entryFailure locates a validation error at the header line of the offending [[table]] entry.
func entryFailure(name string, data []byte, table string, err error) error {
	var entryErr *EntryError
	if !errors.As(err, &entryErr) {
		return failAt(name, 0, err.Error())
	}
	lines := headerLines(data, table)
	msg := err.Error()
	if entryErr.Prior >= 0 && entryErr.Prior < len(lines) {
		msg += fmt.Sprintf(" (first declared at line %d)", lines[entryErr.Prior])
	}
	line := 0
	if entryErr.Index < len(lines) {
		line = lines[entryErr.Index]
	}
	return failAt(name, line, msg)
}

func expandHome(dest, home string) string {
	if dest == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(dest, "~/"); ok {
		return home + "/" + rest
	}
	return dest
}

// headerLines returns the 1-based line of each [[table]] header, in file order.
func headerLines(data []byte, table string) []int {
	header := regexp.MustCompile(`^\s*\[\[\s*` + regexp.QuoteMeta(table) + `\s*\]\]`)
	var lines []int
	for i, line := range strings.Split(string(data), "\n") {
		if header.MatchString(line) {
			lines = append(lines, i+1)
		}
	}
	return lines
}
