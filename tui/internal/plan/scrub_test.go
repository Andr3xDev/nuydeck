package plan

import (
	"strings"
	"testing"
)

func secretsStep(values ...string) WriteSecretsStep {
	step := WriteSecretsStep{Path: "/home/u/.secrets"}
	for _, v := range values {
		step.Entries = append(step.Entries, SecretEntry{Name: "K", Value: Secret(v)})
	}
	return step
}

func TestRedact(t *testing.T) {
	tests := []struct {
		name   string
		steps  []Step
		input  string
		want   string
		absent []string
	}{
		{"replaces the value", []Step{secretsStep("hunter2")}, "token=hunter2 ok", "token=*** ok", []string{"hunter2"}},
		{"longer secret goes before its shorter prefix", []Step{secretsStep("ab", "abc")}, "wrote abc", "wrote ***", []string{"abc", "c"}},
		{"order of entries does not matter", []Step{secretsStep("abc", "ab")}, "wrote abc", "wrote ***", nil},
		{"values from every secrets step", []Step{secretsStep("one"), secretsStep("two")}, "one two", "*** ***", nil},
		{"empty value is not a secret", []Step{secretsStep("")}, "plain text", "plain text", nil},
		{"no secrets leaves the text alone", []Step{CommandStep{Label: "x"}}, "plain text", "plain text", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Redact(SecretScrubber(tt.steps), tt.input)
			if got != tt.want {
				t.Errorf("Redact = %q, want %q", got, tt.want)
			}
			for _, secret := range tt.absent {
				if strings.Contains(got, secret) {
					t.Errorf("output %q still contains %q", got, secret)
				}
			}
		})
	}
}
