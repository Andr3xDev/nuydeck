package services_test

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

func TestParseShow_RealOutputInMixedOrder(t *testing.T) {
	out := lines(
		`Id=docker.service`,
		`LoadState=loaded`,
		`ActiveState=failed`,
		`UnitFileState=enabled`,
		``,
		`Id=bluetooth.service`,
		`ActiveState=inactive`,
		`LoadState=loaded`,
		`UnitFileState=disabled`,
		`Description=ignored key`,
		``,
		`Id=nvidia-resume.service`,
		`LoadState=masked`,
		`ActiveState=inactive`,
		`UnitFileState=masked`,
		``,
		`Id=ghost.service`,
		`LoadState=not-found`,
		`ActiveState=inactive`,
		`UnitFileState=`,
		``,
		`Id=NetworkManager.service`,
		`UnitFileState=enabled`,
		`LoadState=loaded`,
		`ActiveState=active`,
	)
	got := services.ParseShow(out)

	if len(got) != 5 {
		t.Fatalf("ParseShow() returned %d units, want 5: %+v", len(got), got)
	}
	if s := got["NetworkManager.service"]; !s.Found || s.Masked || s.Enabled != services.EnabledEnabled || s.Active != services.ActiveActive {
		t.Errorf("NetworkManager = %+v, want found, enabled, active", s)
	}
	if s := got["docker.service"]; !s.Found || s.Enabled != services.EnabledEnabled || s.Active != services.ActiveFailed {
		t.Errorf("docker = %+v, want found, enabled, failed", s)
	}
	if s := got["bluetooth.service"]; !s.Found || s.Enabled != services.EnabledDisabled || s.Active != services.ActiveInactive {
		t.Errorf("bluetooth = %+v, want found, disabled, inactive", s)
	}
	if s := got["nvidia-resume.service"]; !s.Found || !s.Masked {
		t.Errorf("nvidia-resume = %+v, want found and masked", s)
	}
	if s := got["ghost.service"]; s.Found {
		t.Errorf("ghost = %+v, want Found=false", s)
	}
}

func TestParseShow_UnknownStatesKeepTheirRawValue(t *testing.T) {
	out := lines(
		`Id=cups.service`,
		`LoadState=loaded`,
		`ActiveState=activating`,
		`UnitFileState=static`,
	)

	s := services.ParseShow(out)["cups.service"]

	if string(s.Enabled) != "static" || string(s.Active) != "activating" {
		t.Errorf("state = %+v, want the raw static / activating values, not other", s)
	}
}

func TestParseShow_EmptyOutputYieldsNoUnits(t *testing.T) {
	if got := services.ParseShow(""); len(got) != 0 {
		t.Errorf("ParseShow(\"\") = %+v, want empty", got)
	}
}
