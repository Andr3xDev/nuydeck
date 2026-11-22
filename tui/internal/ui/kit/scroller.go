package kit

// Scroller is the offset of a scrollable text region; it clamps lazily when rendering.
type Scroller struct{ Offset int }

// By moves the offset by delta rows; Window clamps it.
func (s *Scroller) By(delta int) { s.Offset += delta }

// Reset scrolls back to the top.
func (s *Scroller) Reset() { s.Offset = 0 }

// Window returns the h lines visible at the current offset, clamping the offset to the content.
func (s *Scroller) Window(lines []string, h int) []string {
	h = max(h, 0)
	s.Offset = max(min(s.Offset, len(lines)-h), 0)
	return lines[s.Offset:min(s.Offset+h, len(lines))]
}

// WindowAround returns the h lines that keep line index at visible.
func WindowAround(lines []string, at, h int) []string {
	h = max(h, 0)
	start := max(min(at-h/2, len(lines)-h), 0)
	return lines[start:min(start+h, len(lines))]
}
