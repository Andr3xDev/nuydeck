package references

import (
	"path/filepath"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Status is whether the thing a reference points at is installed.
type Status int

const (
	StatusUnknown Status = iota
	StatusPresent
	StatusAbsent
)

// Detect is present when any configured signal is present, absent when all are absent, unknown with no signal or a probe error.
func Detect(ref Reference, hasCommand func(string) bool, pathKind func(string) (plan.NodeKind, error), home string) Status {
	signals := 0
	if ref.DetectBinary != "" {
		signals++
		if hasCommand(ref.DetectBinary) {
			return StatusPresent
		}
	}
	if ref.DetectPath != "" {
		signals++
		path := ref.DetectPath
		if rest, ok := strings.CutPrefix(path, "~/"); ok {
			path = filepath.Join(home, rest)
		}
		kind, err := pathKind(path)
		if err != nil {
			return StatusUnknown
		}
		if kind != plan.KindNone {
			return StatusPresent
		}
	}
	if signals == 0 {
		return StatusUnknown
	}
	return StatusAbsent
}
