package domain_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/pelletier/go-toml/v2"
)

const sentinel = "S3NTINEL-do-not-leak"

func sentinelStep() plan.WriteSecretsStep {
	return plan.WriteSecretsStep{
		Path:    "/home/test/.secrets",
		Entries: []plan.SecretEntry{{Name: "API_TOKEN", Value: plan.Secret(sentinel)}},
	}
}

func TestSecretNeverPrintsItsValue(t *testing.T) {
	step := sentinelStep()
	planned := domain.PlannedStep{Phase: domain.PhaseEnv, Step: step}

	outputs := map[string]string{
		"%v secret":   fmt.Sprintf("%v", step.Entries[0].Value),
		"%s secret":   fmt.Sprintf("%s", step.Entries[0].Value),
		"%q secret":   fmt.Sprintf("%q", step.Entries[0].Value),
		"%v step":     fmt.Sprintf("%v", step),
		"%+v step":    fmt.Sprintf("%+v", step),
		"%#v step":    fmt.Sprintf("%#v", step),
		"%v planned":  fmt.Sprintf("%v", planned),
		"%+v planned": fmt.Sprintf("%+v", planned),
		"%#v planned": fmt.Sprintf("%#v", planned),
	}
	jsonOut, err := json.Marshal(planned)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	outputs["json"] = string(jsonOut)
	tomlOut, err := toml.Marshal(step)
	if err != nil {
		t.Fatalf("toml.Marshal: %v", err)
	}
	outputs["toml"] = string(tomlOut)

	for name, out := range outputs {
		if strings.Contains(out, sentinel) {
			t.Errorf("%s leaked the secret: %s", name, out)
		}
	}
	if got := fmt.Sprintf("%v", step.Entries[0].Value); got != "[redacted]" {
		t.Errorf("%%v = %q, want [redacted]", got)
	}
}
