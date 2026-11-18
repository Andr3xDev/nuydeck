package services_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

var (
	nm       = services.Entry{Unit: "NetworkManager.service", Scope: plan.ScopeSystem, Source: services.SourcePackage, Critical: true}
	bt       = services.Entry{Unit: "bluetooth.service", Scope: plan.ScopeSystem, Source: services.SourcePackage}
	timer    = services.Entry{Unit: "clip.timer", Scope: plan.ScopeUser, Source: services.SourceCustom}
	catalog  = []services.Entry{nm, bt, timer}
	present  = services.UnitState{Found: true}
	allActs  = []plan.Action{plan.ActionEnable, plan.ActionDisable, plan.ActionStart, plan.ActionStop, plan.ActionRestart, plan.ActionMask, plan.ActionUnmask}
	sudoStep = plan.PrivilegeSteps()
)

func request(e services.Entry, a plan.Action) services.Request {
	return services.Request{Unit: e.Unit, Scope: e.Scope, Action: a}
}

func TestPlanAction_RejectsTargetOutsideCatalog(t *testing.T) {
	tests := []struct {
		name string
		req  services.Request
	}{
		{"unlisted unit", services.Request{Unit: "sshd.service", Scope: plan.ScopeSystem, Action: plan.ActionStop}},
		{"listed unit in the other scope", services.Request{Unit: bt.Unit, Scope: plan.ScopeUser, Action: plan.ActionStop}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := services.PlanAction(catalog, tt.req, present, false)
			if !errors.Is(err, services.ErrNotManaged) {
				t.Errorf("error = %v, want ErrNotManaged", err)
			}
		})
	}
}

func TestPlanAction_UnitNotFoundBlocksEveryAction(t *testing.T) {
	for _, action := range allActs {
		_, err := services.PlanAction(catalog, request(bt, action), services.UnitState{}, false)
		if !errors.Is(err, services.ErrUnitNotFound) {
			t.Errorf("%s: error = %v, want ErrUnitNotFound", action, err)
		}
	}
}

func TestPlanAction_SingleServiceActionStepPerAction(t *testing.T) {
	for _, action := range allActs {
		got, err := services.PlanAction(catalog, request(bt, action), present, true)
		if err != nil {
			t.Fatalf("%s: error = %v", action, err)
		}
		want := plan.ServiceActionStep{Unit: bt.Unit, Scope: plan.ScopeSystem, Action: action}
		if len(got.Steps) != 1 || got.Steps[0] != want {
			t.Errorf("%s: Steps = %+v, want [%+v]", action, got.Steps, want)
		}
	}
}

func TestPlanAction_TimerIsTargetedAsItsOwnUnit(t *testing.T) {
	got, err := services.PlanAction(catalog, request(timer, plan.ActionRestart), present, false)
	if err != nil {
		t.Fatal(err)
	}
	want := plan.ServiceActionStep{Unit: "clip.timer", Scope: plan.ScopeUser, Action: plan.ActionRestart}
	if len(got.Steps) != 1 || got.Steps[0] != want {
		t.Errorf("Steps = %+v, want [%+v]", got.Steps, want)
	}
}

func TestPlanAction_PrivilegeStepsBySudoCacheAndScope(t *testing.T) {
	tests := []struct {
		name       string
		entry      services.Entry
		sudoCached bool
		wantSudo   bool
	}{
		{"system, sudo not cached", bt, false, true},
		{"system, sudo cached", bt, true, false},
		{"user, sudo not cached", timer, false, false},
		{"user, sudo cached", timer, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := services.PlanAction(catalog, request(tt.entry, plan.ActionRestart), present, tt.sudoCached)
			if err != nil {
				t.Fatal(err)
			}
			wantSteps := 1
			if tt.wantSudo {
				wantSteps += len(sudoStep)
			}
			if len(got.Steps) != wantSteps {
				t.Fatalf("got %d steps, want %d: %+v", len(got.Steps), wantSteps, got.Steps)
			}
			if tt.wantSudo && !reflect.DeepEqual(got.Steps[:len(sudoStep)], sudoStep) {
				t.Errorf("leading steps = %+v, want PrivilegeSteps() %+v", got.Steps[:len(sudoStep)], sudoStep)
			}
			if _, isAction := got.Steps[len(got.Steps)-1].(plan.ServiceActionStep); !isAction {
				t.Errorf("last step = %T, want ServiceActionStep", got.Steps[len(got.Steps)-1])
			}
		})
	}
}

func TestPlanAction_ExtraConfirmComesFromRisk(t *testing.T) {
	tests := []struct {
		name   string
		entry  services.Entry
		action plan.Action
	}{
		{"critical stop", nm, plan.ActionStop},
		{"critical restart", nm, plan.ActionRestart},
		{"plain mask", bt, plan.ActionMask},
		{"plain restart", bt, plan.ActionRestart},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := services.PlanAction(catalog, request(tt.entry, tt.action), present, true)
			if err != nil {
				t.Fatal(err)
			}
			if want := services.Risk(tt.entry, tt.action); got.ExtraConfirm != want {
				t.Errorf("ExtraConfirm = %q, want %q", got.ExtraConfirm, want)
			}
		})
	}
}
