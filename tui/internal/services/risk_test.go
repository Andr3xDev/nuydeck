package services_test

import (
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

func TestRisk_CriticalWordingNamesTheConsequenceAndWhenItBites(t *testing.T) {
	critical := services.Entry{Unit: "NetworkManager.service", Scope: plan.ScopeSystem, Critical: true}
	tests := []struct {
		action  plan.Action
		want    []string
		notWant []string
	}{
		{plan.ActionMask, []string{"network", "session", "blocks it from starting"}, nil},
		{plan.ActionDisable, []string{"network", "session", "next boot"}, nil},
		{plan.ActionStop, []string{"network", "session", "now"}, []string{"next boot"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			got := strings.ToLower(services.Risk(critical, tt.action))
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("Risk() = %q, want it to contain %q", got, w)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(got, w) {
					t.Errorf("Risk() = %q, must not contain %q", got, w)
				}
			}
		})
	}
}

func TestRisk(t *testing.T) {
	critical := services.Entry{Unit: "NetworkManager.service", Scope: plan.ScopeSystem, Critical: true}
	plain := services.Entry{Unit: "bluetooth.service", Scope: plan.ScopeSystem}

	tests := []struct {
		name        string
		entry       services.Entry
		action      plan.Action
		wantWarning bool
	}{
		{"critical stop", critical, plan.ActionStop, true},
		{"critical disable", critical, plan.ActionDisable, true},
		{"critical mask", critical, plan.ActionMask, true},
		{"critical restart", critical, plan.ActionRestart, false},
		{"critical start", critical, plan.ActionStart, false},
		{"critical enable", critical, plan.ActionEnable, false},
		{"critical unmask", critical, plan.ActionUnmask, false},
		{"plain mask", plain, plan.ActionMask, true},
		{"plain stop", plain, plan.ActionStop, false},
		{"plain disable", plain, plan.ActionDisable, false},
		{"plain restart", plain, plan.ActionRestart, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := services.Risk(tt.entry, tt.action)
			if (got != "") != tt.wantWarning {
				t.Fatalf("Risk() = %q, want warning = %v", got, tt.wantWarning)
			}
			if tt.wantWarning && !strings.Contains(got, tt.entry.Unit) {
				t.Errorf("Risk() = %q, want it to name %q", got, tt.entry.Unit)
			}
		})
	}
}
