package envvars

import (
	"bytes"
	"errors"
)

// ErrLineChanged means the line is no longer the row that was shown.
var ErrLineChanged = errors.New("the env line changed")

// ToggleChange holds the target line before and after the toggle, for the preview.
type ToggleChange struct {
	Before string
	After  string
}

// ToggleLine comments out or uncomments the line of v, leaving every other byte as it was.
func ToggleLine(content []byte, v Var) ([]byte, ToggleChange, error) {
	lines := bytes.Split(content, []byte("\n"))
	if v.Line < 0 || v.Line >= len(lines) {
		return nil, ToggleChange{}, ErrLineChanged
	}
	before := string(lines[v.Line])
	current, indentEnd, markEnd, ok := parseLine(before)
	current.Line = v.Line
	if !ok || current != v {
		return nil, ToggleChange{}, ErrLineChanged
	}
	after := before[:indentEnd] + "-- " + before[indentEnd:]
	if v.Off {
		after = before[:indentEnd] + before[markEnd:]
	}
	lines[v.Line] = []byte(after)
	return bytes.Join(lines, []byte("\n")), ToggleChange{Before: before, After: after}, nil
}
