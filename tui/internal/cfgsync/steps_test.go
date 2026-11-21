package cfgsync

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestDirectionName(t *testing.T) {
	cases := map[plan.Direction]string{
		plan.Push:    "push",
		plan.Pull:    "pull",
		plan.Restore: "restore",
		plan.Manual:  "manual",
	}
	for dir, want := range cases {
		if got := directionName(dir); got != want {
			t.Errorf("directionName(%d) = %q, want %q", dir, got, want)
		}
	}
}
