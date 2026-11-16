package plan_test

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const sentinel = "S3NTINEL-do-not-leak"

func TestSecretRevealReturnsValue(t *testing.T) {
	if got := plan.Secret(sentinel).Reveal(); got != sentinel {
		t.Fatalf("Reveal() = %q", got)
	}
}
