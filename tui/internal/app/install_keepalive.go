package app

import (
	"cmp"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// defaultKeepaliveEvery stays well under sudo's default 5 minute timestamp_timeout.
const defaultKeepaliveEvery = 60 * time.Second

// startKeepalive refreshes the sudo timestamp until ctx ends. A failed refresh does not abort the running step;
// it only fails the next step that needs sudo.
func (i *Installer) startKeepalive(ctx context.Context, wg *sync.WaitGroup, expired *atomic.Bool) {
	ticker := time.NewTicker(cmp.Or(i.KeepaliveEvery, defaultKeepaliveEvery))
	probe := sudoProbeStep("keep sudo alive")
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if i.Runner.Run(ctx, probe, nil).Status == plan.StepFailed {
					expired.Store(true)
				}
			}
		}
	}()
}
