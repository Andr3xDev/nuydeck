package services_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
)

func TestLogsCommand_ArgvPerScope(t *testing.T) {
	tests := []struct {
		name     string
		entry    services.Entry
		wantUser bool
	}{
		{"system", services.Entry{Unit: "docker.service", Scope: plan.ScopeSystem}, false},
		{"user", services.Entry{Unit: "clip.timer", Scope: plan.ScopeUser}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := services.LogsCommand(tt.entry)
			if err != nil {
				t.Fatalf("LogsCommand() error = %v", err)
			}
			args := cmd.Args
			if len(args) < 3 || args[0] != "journalctl" || !slices.Contains(args, "-e") ||
				!slices.Equal(args[len(args)-2:], []string{"-u", tt.entry.Unit}) {
				t.Fatalf("Args = %q, want journalctl with -e ending in -u %s", args, tt.entry.Unit)
			}
			user := slices.Index(args, "--user")
			if tt.wantUser != (user >= 0) || (tt.wantUser && user > len(args)-3) {
				t.Errorf("Args = %q, want --user = %v placed before -u", args, tt.wantUser)
			}
		})
	}
}

// The entry is the only input, so "outside the catalog" is expressed as an entry whose unit
// fails the catalog unit pattern (a free-typed name can never be a valid catalog row).
func TestLogsCommand_RejectsUnitOutsideCatalogShape(t *testing.T) {
	for _, unit := range []string{"", "-f", "--user", "a b.service", "x.socket"} {
		_, err := services.LogsCommand(services.Entry{Unit: unit, Scope: plan.ScopeSystem})
		if !errors.Is(err, services.ErrNotManaged) {
			t.Errorf("LogsCommand(%q) error = %v, want ErrNotManaged", unit, err)
		}
	}
}
