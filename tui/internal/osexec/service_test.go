package osexec_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestServiceActionStep_ArgvPerScope(t *testing.T) {
	cases := []struct {
		name     string
		step     plan.ServiceActionStep
		wantName string
		wantArgs []string
	}{
		{
			"system goes through non-interactive sudo",
			plan.ServiceActionStep{Unit: "NetworkManager.service", Scope: plan.ScopeSystem, Action: plan.ActionRestart},
			"sudo", []string{"-n", "--", "systemctl", "restart", "NetworkManager.service"},
		},
		{
			"user runs without sudo",
			plan.ServiceActionStep{Unit: "lucyna-clipboard-wipe.timer", Scope: plan.ScopeUser, Action: plan.ActionEnable},
			"systemctl", []string{"--user", "enable", "lucyna-clipboard-wipe.timer"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeExec{}
			if res := newRunner(f).Run(context.Background(), tc.step, nil); res.Status != plan.StepOK {
				t.Fatalf("result = %+v", res)
			}
			if len(f.calls) != 1 || f.calls[0].name != tc.wantName || !reflect.DeepEqual(f.calls[0].args, tc.wantArgs) {
				t.Errorf("calls = %+v, want %s %v", f.calls, tc.wantName, tc.wantArgs)
			}
		})
	}
}

func TestServiceActionStep_NonZeroExitNamesUnitActionAndStderr(t *testing.T) {
	f := &fakeExec{failOn: map[string]bool{"systemctl": true}}
	step := plan.ServiceActionStep{Unit: "ly@tty1.service", Scope: plan.ScopeSystem, Action: plan.ActionStop}
	res := newRunner(f).Run(context.Background(), step, nil)
	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("result = %+v, want failed", res)
	}
	for _, want := range []string{"ly@tty1.service", "stop", "boom"} {
		if !strings.Contains(res.Err.Error(), want) {
			t.Errorf("error %q lacks %q", res.Err, want)
		}
	}
}

func TestServiceActionStep_InvalidUnitIsRefusedBeforeAnyExec(t *testing.T) {
	f := &fakeExec{}
	step := plan.ServiceActionStep{Unit: "--now.service", Scope: plan.ScopeSystem, Action: plan.ActionStart}
	res := newRunner(f).Run(context.Background(), step, nil)
	if res.Status != plan.StepFailed || len(f.calls) != 0 {
		t.Errorf("result = %+v, calls = %+v, want failed with no exec", res, f.calls)
	}
}

func TestConfirmStep_IsUnsupportedByTheRunner(t *testing.T) {
	f := &fakeExec{}
	res := newRunner(f).Run(context.Background(), plan.ConfirmStep{ID: "spotify", Prompt: "Did Spotify open?"}, nil)
	if res.Status != plan.StepFailed || res.Err == nil || !strings.Contains(res.Err.Error(), "unsupported step") || len(f.calls) != 0 {
		t.Errorf("result = %+v, calls = %+v", res, f.calls)
	}
}
