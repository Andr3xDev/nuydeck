package tomlx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

var entryHeader = regexp.MustCompile(`^\s*\[\[entry\]\]`)

// LoadManifest reads and validates the manifest; home expands the leading ~ of root destinations.
// Errors are prefixed with the file's base name and the offending line when known.
func LoadManifest(path, home string) (domain.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Manifest{}, err
	}
	name := filepath.Base(path)
	fail := func(line int, msg string) error {
		if line == 0 {
			return fmt.Errorf("%s: %s", name, msg)
		}
		return fmt.Errorf("%s:%d: %s", name, line, msg)
	}

	var m domain.Manifest
	dec := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		var strict *toml.StrictMissingError
		var decodeErr *toml.DecodeError
		switch {
		case errors.As(err, &strict) && len(strict.Errors) > 0:
			first := strict.Errors[0]
			line, _ := first.Position()
			return domain.Manifest{}, fail(line, fmt.Sprintf("unknown key %q", strings.Join(first.Key(), ".")))
		case errors.As(err, &decodeErr):
			line, _ := decodeErr.Position()
			return domain.Manifest{}, fail(line, decodeErr.Error())
		}
		return domain.Manifest{}, fail(0, err.Error())
	}

	for root, dest := range m.Roots {
		m.Roots[root] = expandHome(dest, home)
	}
	if err := m.Validate(home); err != nil {
		var entryErr *domain.EntryError
		if !errors.As(err, &entryErr) {
			return domain.Manifest{}, fail(0, err.Error())
		}
		lines := entryHeaderLines(data)
		msg := err.Error()
		if entryErr.Prior >= 0 {
			msg += fmt.Sprintf(" (first declared at line %d)", lines[entryErr.Prior])
		}
		return domain.Manifest{}, fail(lines[entryErr.Index], msg)
	}
	return m, nil
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

// entryHeaderLines returns the 1-based line of each [[entry]] header, in file order.
func entryHeaderLines(data []byte) []int {
	var lines []int
	for i, line := range strings.Split(string(data), "\n") {
		if entryHeader.MatchString(line) {
			lines = append(lines, i+1)
		}
	}
	return lines
}
