package osexec

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// openLinkGrace is how long xdg-open may run before it counts as a handler that stays open (a browser).
var openLinkGrace = 2 * time.Second

// OpenURL opens an https URL with xdg-open, with stdio discarded and no shell.
// The handler is not bound to ctx and runs in its own session, so a browser it starts outlives the app;
// ctx only bounds the wait for an early exit.
func OpenURL(ctx context.Context, url string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("refusing to open non-https url %q", url)
	}
	cmd := exec.Command("xdg-open", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("xdg-open: %w", err)
	}
	// Wait always runs, so the child is reaped even when we stop waiting for it.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		if err != nil {
			return fmt.Errorf("xdg-open: %w", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(openLinkGrace):
	}
	return nil
}
