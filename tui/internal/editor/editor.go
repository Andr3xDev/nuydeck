package editor

import (
	"errors"
	"strings"
)

const fallbackEditor = "nano"

var errNoEditor = errors.New("no editor: set $VISUAL or $EDITOR or install nano")

// Resolve returns the editor argv from $VISUAL, then $EDITOR, then nano if installed.
func Resolve(getenv func(string) string, hasCommand func(string) bool) ([]string, error) {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if argv := strings.Fields(getenv(name)); len(argv) > 0 {
			return argv, nil
		}
	}
	if hasCommand(fallbackEditor) {
		return []string{fallbackEditor}, nil
	}
	return nil, errNoEditor
}
