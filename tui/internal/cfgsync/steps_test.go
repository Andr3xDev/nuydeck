package cfgsync

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestDirectionName(t *testing.T) {
	if got := directionName(plan.Manual); got != "manual" {
		t.Errorf("directionName(Manual) = %q, want %q", got, "manual")
	}
}
