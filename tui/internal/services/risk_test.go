package services_test

import (
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

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
