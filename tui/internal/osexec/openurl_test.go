package osexec_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/osexec"
)

const openLinkTestURL = "https://example.org/docs"

// fakeXdgOpen puts an executable xdg-open on an otherwise empty PATH; the script records its argv in args.
func fakeXdgOpen(t *testing.T, body string) (argsFile string) {
	t.Helper()
	bin := t.TempDir()
	argsFile = filepath.Join(bin, "args")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + argsFile + "\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, "xdg-open"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	return argsFile
}

func TestOpenURL_SuccessRunsXdgOpenWithTheURLOnly(t *testing.T) {
	argsFile := fakeXdgOpen(t, "exit 0")
	if err := osexec.OpenURL(context.Background(), openLinkTestURL); err != nil {
		t.Fatalf("OpenURL() error = %v", err)
	}
	if got, _ := os.ReadFile(argsFile); string(got) != openLinkTestURL {
		t.Errorf("xdg-open args = %q, want %q", got, openLinkTestURL)
	}
}

func TestOpenURL_NonZeroExitWithinGraceIsReturned(t *testing.T) {
	fakeXdgOpen(t, "exit 3")
	if err := osexec.OpenURL(context.Background(), openLinkTestURL); err == nil {
		t.Error("OpenURL() = nil, want the exit error")
	}
}

func TestOpenURL_StillRunningAfterGraceIsSuccess(t *testing.T) {
	t.Cleanup(osexec.SetOpenLinkGrace(50 * time.Millisecond))
	fakeXdgOpen(t, "exec sleep 1")
	start := time.Now()
	if err := osexec.OpenURL(context.Background(), openLinkTestURL); err != nil {
		t.Fatalf("OpenURL() error = %v, want nil for a handler still running", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("OpenURL blocked %v, want it to return after the grace window", elapsed)
	}
}

func TestOpenURL_MissingXdgOpenIsAClearError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := osexec.OpenURL(context.Background(), openLinkTestURL)
	if err == nil || !strings.Contains(err.Error(), "xdg-open") {
		t.Errorf("err = %v, want an error naming xdg-open", err)
	}
}

func TestOpenURL_NonHTTPSIsRefusedWithoutSpawning(t *testing.T) {
	argsFile := fakeXdgOpen(t, "exit 0")
	for _, url := range []string{"http://example.org", "file:///etc/passwd", "--help", ""} {
		if err := osexec.OpenURL(context.Background(), url); err == nil {
			t.Errorf("OpenURL(%q) = nil, want refusal", url)
		}
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Error("xdg-open was spawned for a refused url")
	}
}
