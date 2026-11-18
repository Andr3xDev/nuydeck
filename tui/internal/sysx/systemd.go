package sysx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

// SystemctlProbe is the real services.StateProbe. A nil Run means execx.Run.
type SystemctlProbe struct {
	Run RunFunc
}

func (p SystemctlProbe) Read(ctx context.Context, scope plan.Scope, units []string) (map[string]services.UnitState, error) {
	args := []string{"show", "--no-pager", "-p", "Id,LoadState,UnitFileState,ActiveState"}
	if scope == plan.ScopeUser {
		if !userBusExists() {
			return nil, services.ErrNoUserBus
		}
		args = append([]string{"--user"}, args...)
	}
	run := p.Run
	if run == nil {
		run = execx.Run
	}
	var out bytes.Buffer
	res, err := run(ctx, &out, "systemctl", append(args, units...)...)
	if err != nil {
		return nil, fmt.Errorf("systemctl: %w", err)
	}
	if res.ExitCode != 0 {
		firstLine, _, _ := strings.Cut(strings.TrimSpace(res.Stderr), "\n")
		return nil, fmt.Errorf("systemctl exited with %d: %s", res.ExitCode, firstLine)
	}
	shown := services.ParseShowBlocks(out.String())
	if len(shown) != len(units) {
		return services.ParseShow(out.String()), nil
	}
	// An alias prints its target's Id, so key by the requested name.
	states := make(map[string]services.UnitState, len(units))
	for i, unit := range units {
		states[unit] = shown[i].State
	}
	return states, nil
}

func userBusExists() bool {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "bus"))
	return err == nil
}
