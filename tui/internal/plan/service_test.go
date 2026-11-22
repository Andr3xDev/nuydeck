package plan

import (
	"reflect"
	"testing"
)

func TestServiceActionStepCommand_BuildsLabelArgvAndSudoPerScope(t *testing.T) {
	tests := []struct {
		name string
		step ServiceActionStep
		want CommandStep
	}{
		{
			"system restart",
			ServiceActionStep{Unit: "NetworkManager.service", Scope: ScopeSystem, Action: ActionRestart},
			CommandStep{Label: "restart NetworkManager.service (system)", Argv: []string{"systemctl", "restart", "NetworkManager.service"}, Sudo: true},
		},
		{
			"user disable",
			ServiceActionStep{Unit: "lucyna-clipboard-wipe.timer", Scope: ScopeUser, Action: ActionDisable},
			CommandStep{Label: "disable lucyna-clipboard-wipe.timer (user)", Argv: []string{"systemctl", "--user", "disable", "lucyna-clipboard-wipe.timer"}},
		},
		{
			"template instance",
			ServiceActionStep{Unit: "ly@tty1.service", Scope: ScopeSystem, Action: ActionEnable},
			CommandStep{Label: "enable ly@tty1.service (system)", Argv: []string{"systemctl", "enable", "ly@tty1.service"}, Sudo: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.step.Command()
			if err != nil {
				t.Fatalf("Command() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Command() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestServiceActionStepCommand_UsesVerbOfEveryAction(t *testing.T) {
	actions := []Action{ActionEnable, ActionDisable, ActionStart, ActionStop, ActionRestart, ActionMask, ActionUnmask}
	for _, a := range actions {
		got, err := ServiceActionStep{Unit: "a.service", Scope: ScopeSystem, Action: a}.Command()
		if err != nil {
			t.Fatalf("%q: Command() error = %v", a, err)
		}
		if got.Argv[1] != string(a) {
			t.Errorf("argv = %v, want verb %q", got.Argv, a)
		}
	}
}

func TestServiceActionStepCommand_RefusesInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		step ServiceActionStep
	}{
		{"unknown action", ServiceActionStep{Unit: "a.service", Scope: ScopeSystem, Action: Action("reboot")}},
		{"empty action", ServiceActionStep{Unit: "a.service", Scope: ScopeSystem}},
		{"unknown scope", ServiceActionStep{Unit: "a.service", Scope: Scope("global"), Action: ActionStart}},
		{"leading dash", ServiceActionStep{Unit: "--now.service", Scope: ScopeSystem, Action: ActionStart}},
		{"no suffix", ServiceActionStep{Unit: "a", Scope: ScopeSystem, Action: ActionStart}},
		{"socket suffix", ServiceActionStep{Unit: "a.socket", Scope: ScopeSystem, Action: ActionStart}},
		{"space", ServiceActionStep{Unit: "a b.service", Scope: ScopeSystem, Action: ActionStart}},
		{"slash", ServiceActionStep{Unit: "../a.service", Scope: ScopeSystem, Action: ActionStart}},
		{"empty unit", ServiceActionStep{Scope: ScopeSystem, Action: ActionStart}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.step.Command(); err == nil {
				t.Errorf("Command() error = nil, want a refusal for %+v", tt.step)
			}
		})
	}
}

func TestDescribe_ServiceActionAndConfirmSteps(t *testing.T) {
	tests := []struct {
		step      Step
		wantLabel string
		wantKind  string
	}{
		{ServiceActionStep{Unit: "NetworkManager.service", Scope: ScopeSystem, Action: ActionRestart}, "restart NetworkManager.service (system)", "service"},
		{ConfirmStep{ID: "spotify", Prompt: "Did Spotify open?"}, "Did Spotify open?", "confirm"},
	}
	for _, tt := range tests {
		label, kind := Describe(tt.step)
		if label != tt.wantLabel || kind != tt.wantKind {
			t.Errorf("Describe(%T) = (%q, %q), want (%q, %q)", tt.step, label, kind, tt.wantLabel, tt.wantKind)
		}
	}
}
