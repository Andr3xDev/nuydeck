package install_test

import (
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
)

// Excerpt of the stock Arch /etc/pacman.conf; multilib-testing sits right above multilib and must stay untouched.
const pacmanConfHead = `[options]
HoldPkg     = pacman glibc
Architecture = auto

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist

#[core-testing]
#Include = /etc/pacman.d/mirrorlist

#[multilib-testing]
#Include = /etc/pacman.d/mirrorlist

`

func TestEnableMultilib(t *testing.T) {
	tests := []struct {
		name        string
		multilib    string
		wantChanged bool
		wantErr     bool
		wantActive  string
	}{
		{"commented", "#[multilib]\n#Include = /etc/pacman.d/mirrorlist\n", true, false,
			"[multilib]\nInclude = /etc/pacman.d/mirrorlist\n"},
		{"already active", "[multilib]\nInclude = /etc/pacman.d/mirrorlist\n", false, false,
			"[multilib]\nInclude = /etc/pacman.d/mirrorlist\n"},
		{"trailing whitespace", "#[multilib]  \n#Include = /etc/pacman.d/mirrorlist\t\n", true, false,
			"[multilib]\nInclude = /etc/pacman.d/mirrorlist\n"},
		{"crlf line endings are rejected", "#[multilib]\r\n#Include = /etc/pacman.d/mirrorlist\r\n", false, true, ""},
		{"absent", "", false, true, ""},
		{"header without include", "#[multilib]\n#Server = https://example.org\n", false, true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := pacmanConfHead + tc.multilib
			out, changed, err := install.EnableMultilib([]byte(in))

			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "pacman.conf") {
					t.Fatalf("err = %v, want error naming pacman.conf", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if !tc.wantChanged {
				if string(out) != in {
					t.Errorf("unchanged input was rewritten:\n%s", out)
				}
				return
			}
			if string(out) != pacmanConfHead+tc.wantActive {
				t.Errorf("output = %q, want %q", out, pacmanConfHead+tc.wantActive)
			}
		})
	}
}

func TestEnableMultilibUncommentsOnlyTheTwoLines(t *testing.T) {
	in := pacmanConfHead + "#[multilib]\n#Include = /etc/pacman.d/mirrorlist\n"
	out, _, err := install.EnableMultilib([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := strings.Replace(in, "#[multilib]\n#Include", "[multilib]\nInclude", 1); string(out) != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}
