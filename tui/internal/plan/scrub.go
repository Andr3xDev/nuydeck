package plan

import (
	"slices"
	"strings"
)

const secretMask = "***"

// SecretScrubber masks every secret value of the steps; values never reach argv or output by design, this is the second line.
func SecretScrubber(steps []Step) *strings.Replacer {
	var values []string
	for _, s := range steps {
		if w, ok := s.(WriteSecretsStep); ok {
			for _, e := range w.Entries {
				if e.Value != "" {
					values = append(values, e.Value.Reveal())
				}
			}
		}
	}
	// NewReplacer matches in argument order, so a longer value must come before its prefix.
	slices.SortStableFunc(values, func(a, b string) int { return len(b) - len(a) })
	pairs := make([]string, 0, 2*len(values))
	for _, v := range values {
		pairs = append(pairs, v, secretMask)
	}
	return strings.NewReplacer(pairs...)
}

// Redact masks the secret values the scrubber knows.
func Redact(scrubber *strings.Replacer, text string) string {
	return scrubber.Replace(text)
}
