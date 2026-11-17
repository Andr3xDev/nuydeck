package dryrun

import (
	"context"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// Guards D29: an edited file is shown by size, never by content.
func TestRecorder_WriteFileStepShowsByteCountNotContent(t *testing.T) {
	step := plan.WriteFileStep{Path: "/home/u/.config/hypr/a.conf", Content: []byte("TOKEN=hunter2\n"), Mode: 0o644, Root: "/home/u/.config/hypr"}

	got := Recorder{}.Run(context.Background(), step, nil).Detail

	if want := "would write /home/u/.config/hypr/a.conf (14 bytes)"; got != want {
		t.Errorf("detail = %q, want %q", got, want)
	}
	if strings.Contains(got, "hunter2") {
		t.Errorf("detail leaks content: %q", got)
	}
}
