package ui

// scroller is the offset of a scrollable text region; it clamps lazily when rendering.
type scroller struct{ offset int }

func (s *scroller) by(delta int) { s.offset += delta }

func (s *scroller) reset() { s.offset = 0 }

// window returns the h lines visible at the current offset, clamping the offset to the content.
func (s *scroller) window(lines []string, h int) []string {
	h = max(h, 0)
	s.offset = max(min(s.offset, len(lines)-h), 0)
	return lines[s.offset:min(s.offset+h, len(lines))]
}

// windowAround returns the h lines that keep line index at visible.
func windowAround(lines []string, at, h int) []string {
	h = max(h, 0)
	start := max(min(at-h/2, len(lines)-h), 0)
	return lines[start:min(start+h, len(lines))]
}
