package editor

import (
	"errors"
	"strings"
)

var fallbackEditors = []string{"nvim", "vim", "nano", "vi"}

var errNoEditor = errors.New("no editor: set $VISUAL or $EDITOR or install nvim, vim, nano or vi")

// Resolve returns the editor argv from $VISUAL, then $EDITOR, then the first installed fallback editor.
func Resolve(getenv func(string) string, hasCommand func(string) bool) ([]string, error) {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if argv := strings.Fields(getenv(name)); len(argv) > 0 {
			return argv, nil
		}
	}
	for _, name := range fallbackEditors {
		if hasCommand(name) {
			return []string{name}, nil
		}
	}
	return nil, errNoEditor
}
