package osexec

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// lineByteCap bounds one emitted line so a runaway process cannot flood the UI.
const lineByteCap = 512

// rawBufferCap bounds the held unterminated line. It is larger than lineByteCap
// so escape sequences inside a still-short visible line are not cut apart.
const rawBufferCap = 4 * lineByteCap

// ansiEscape matches OSC (progress, hyperlinks), CSI (colors, cursor moves),
// charset designators and other two-byte ESC sequences.
var ansiEscape = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b[()][A-Za-z0-9]|\x1b[@-Z\\-_]`)

// lineWriter turns raw process output into sanitized plan.StepProgress lines.
// It is not safe for concurrent use; use one writer per stream.
type lineWriter struct {
	progress    func(plan.StepProgress)
	buf         []byte
	discardRest bool
}

// newLineWriter returns a writer that reports lines to progress, which may be nil.
func newLineWriter(progress func(plan.StepProgress)) *lineWriter {
	if progress == nil {
		progress = func(plan.StepProgress) {}
	}
	return &lineWriter{progress: progress}
}

// Write splits on \n and \r, holding the trailing partial line until it ends.
func (w *lineWriter) Write(p []byte) (int, error) {
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
func (w *lineWriter) Flush() {
	w.endLine()
	w.buf = nil
}

func (w *lineWriter) hold(b []byte) {
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

func (w *lineWriter) endLine() {
	if !w.discardRest {
		w.emit(w.buf)
	}
	w.buf = w.buf[:0]
	w.discardRest = false
}

func (w *lineWriter) emit(raw []byte) {
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
	w.progress(plan.StepProgress{Line: line})
}
