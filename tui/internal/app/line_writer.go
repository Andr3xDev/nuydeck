package app

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// lineByteCap bounds one emitted line so a runaway process cannot flood the UI.
const lineByteCap = 512

// rawBufferCap bounds the held unterminated line. It is larger than lineByteCap
// so escape sequences inside a still-short visible line are not cut apart.
const rawBufferCap = 4 * lineByteCap

// ansiEscape matches OSC (progress, hyperlinks), CSI (colors, cursor moves),
// charset designators and other two-byte ESC sequences.
var ansiEscape = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b[()][A-Za-z0-9]|\x1b[@-Z\\-_]`)

// LineWriter turns raw process output into sanitized StepProgress lines.
// It is not safe for concurrent use; use one writer per stream.
type LineWriter struct {
	progress    func(domain.StepProgress)
	buf         []byte
	discardRest bool
}

// NewLineWriter returns a writer that reports lines to progress, which may be nil.
func NewLineWriter(progress func(domain.StepProgress)) *LineWriter {
	if progress == nil {
		progress = func(domain.StepProgress) {}
	}
	return &LineWriter{progress: progress}
}

// Write splits on \n and \r, holding the trailing partial line until it ends.
func (w *LineWriter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexAny(p, "\r\n")
		if i < 0 {
			w.hold(p)
			break
		}
		w.hold(p[:i])
		w.endLine()
		p = p[i+1:]
	}
	return n, nil
}

// Flush emits the held partial line, if any.
func (w *LineWriter) Flush() {
	w.endLine()
	w.buf = nil
}

func (w *LineWriter) hold(b []byte) {
	if w.discardRest {
		return
	}
	w.buf = append(w.buf, b...)
	if len(w.buf) > rawBufferCap {
		w.emit(w.buf)
		w.buf = nil
		w.discardRest = true
	}
}

func (w *LineWriter) endLine() {
	if !w.discardRest {
		w.emit(w.buf)
	}
	w.buf = w.buf[:0]
	w.discardRest = false
}

func (w *LineWriter) emit(raw []byte) {
	line := strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, ansiEscape.ReplaceAllString(string(raw), ""))
	if line == "" {
		return
	}
	if len(line) > lineByteCap {
		cut := lineByteCap
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		line = line[:cut]
	}
	w.progress(domain.StepProgress{Line: line})
}
