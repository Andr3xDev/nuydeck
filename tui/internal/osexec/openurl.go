package osexec

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// openLinkGrace is how long xdg-open may run before it counts as a handler that stays open (a browser).
const openLinkGrace = 2 * time.Second

// OpenURL opens an https URL with xdg-open, with stdio discarded and no shell.
func OpenURL(ctx context.Context, url string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("refusing to open non-https url %q", url)
	}
	cmd := exec.CommandContext(ctx, "xdg-open", url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("xdg-open: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		if err != nil {
			return fmt.Errorf("xdg-open: %w", err)
		}
	case <-time.After(openLinkGrace):
	}
	return nil
}
