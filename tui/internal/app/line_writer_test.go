package app

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func collectLines(t *testing.T, chunks ...string) []string {
	t.Helper()
	var lines []string
	w := NewLineWriter(func(p domain.StepProgress) { lines = append(lines, p.Line) })
	for _, c := range chunks {
		if _, err := w.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	return lines
}

func TestLineWriterSplitsAndSanitizes(t *testing.T) {
	cases := []struct {
		name   string
		chunks []string
		want   []string
	}{
		{"newline lines", []string{"one\ntwo\n"}, []string{"one", "two"}},
		{"carriage return progress", []string{"10%\r20%\r30%\n"}, []string{"10%", "20%", "30%"}},
		{"crlf yields no empty line", []string{"a\r\nb\n"}, []string{"a", "b"}},
		{"ansi stripped", []string{"\x1b[31mred\x1b[0m ok\n"}, []string{"red ok"}},
		{"osc progress stripped", []string{"\x1b]9;4;1;50\x07installing\n"}, []string{"installing"}},
		{"osc hyperlink stripped", []string{"\x1b]8;;https://example.com/pkg\x1b\\link\x1b]8;;\x1b\\ text\n"}, []string{"link text"}},
		{"charset designator stripped", []string{"\x1b(Bplain\x1b)0\n"}, []string{"plain"}},
		{"two byte escape stripped", []string{"a\x1bMb\n"}, []string{"ab"}},
		{"control bytes stripped", []string{"a\x07b\x00c\n"}, []string{"abc"}},
		{"line split across writes", []string{"hel", "lo\nwor", "ld\n"}, []string{"hello", "world"}},
		{"partial last line flushed", []string{"done\ntail"}, []string{"done", "tail"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := collectLines(t, tc.chunks...); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLineWriterHoldsPartialLineUntilTerminated(t *testing.T) {
	var lines []string
	w := NewLineWriter(func(p domain.StepProgress) { lines = append(lines, p.Line) })

	if _, err := w.Write([]byte("no newline yet")); err != nil {
		t.Fatal(err)
	}

	if len(lines) != 0 {
		t.Errorf("emitted %q before the line ended", lines)
	}
}

func TestLineWriterCapsLongLines(t *testing.T) {
	got := collectLines(t, strings.Repeat("a", 2000)+"\n")

	if len(got) == 0 {
		t.Fatal("long line dropped entirely")
	}
	for _, l := range got {
		if len(l) > lineByteCap {
			t.Errorf("line of %d bytes exceeds the %d cap", len(l), lineByteCap)
		}
	}
}

func TestLineWriterCapsAtRuneBoundary(t *testing.T) {
	got := collectLines(t, strings.Repeat("a", lineByteCap-1)+"éé\n")

	if len(got) != 1 || got[0] != strings.Repeat("a", lineByteCap-1) {
		t.Fatalf("got %d lines, want one cut before the split rune", len(got))
	}
	if !utf8.ValidString(got[0]) {
		t.Error("line cut inside a multibyte rune")
	}
}

func TestLineWriterDiscardsRestOfUnterminatedRunawayLine(t *testing.T) {
	var lines []string
	w := NewLineWriter(func(p domain.StepProgress) { lines = append(lines, p.Line) })
	chunk := []byte(strings.Repeat("a", 1000))

	for range 20 {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if len(lines) != 1 || len(lines[0]) != lineByteCap {
		t.Fatalf("got %d lines before the terminator, want one capped line", len(lines))
	}
	if len(w.buf) != 0 {
		t.Errorf("held %d bytes while discarding", len(w.buf))
	}
	if _, err := w.Write([]byte("tail\nnext\n")); err != nil {
		t.Fatal(err)
	}
	w.Flush()

	if want := []string{lines[0], "next"}; !reflect.DeepEqual(lines, want) {
		t.Errorf("got %d lines %q, want the rest of the runaway line dropped", len(lines), lines[1:])
	}
}

func TestLineWriterNilProgressIsNoop(t *testing.T) {
	w := NewLineWriter(nil)

	if _, err := w.Write([]byte("x\ny")); err != nil {
		t.Fatal(err)
	}
	w.Flush()
}
