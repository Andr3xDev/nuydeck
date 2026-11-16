// Package sysx: read-only system probes (lspci, pacman queries, getent, allow-listed system files).
package sysx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/execx"
	"github.com/Andr3xDev/nuydeck/tui/internal/install"
)

const (
	pacmanConfPath   = "/etc/pacman.conf"
	logindDropInDir  = "/etc/systemd/logind.conf.d"
	passwdShellField = 6
)

// ErrPathNotAllowed is returned by ReadSystemFile for paths outside the allow-list.
var ErrPathNotAllowed = errors.New("path not allowed")

// RunFunc is the exec seam; execx.Run satisfies it.
type RunFunc func(ctx context.Context, out io.Writer, name string, args ...string) (execx.Result, error)

// Probe is the real read-only system probe. A nil Run means execx.Run.
type Probe struct {
	Run  RunFunc
	User string
}

func (p Probe) output(ctx context.Context, name string, args ...string) (string, error) {
	run := p.Run
	if run == nil {
		run = execx.Run
	}
	var out bytes.Buffer
	res, err := run(ctx, &out, name, args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("%s exited with %d: %s", name, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return out.String(), nil
}

func (p Probe) DetectGPUs(ctx context.Context) ([]install.PCIDevice, error) {
	out, err := p.output(ctx, "lspci", "-vmmnn")
	if err != nil {
		return nil, err
	}
	return install.ParseLspci(out), nil
}

func (p Probe) InstalledPackages(ctx context.Context) (map[string]bool, error) {
	names, err := p.packageNames(ctx, "-Qq")
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set, nil
}

func (p Probe) ExplicitPackages(ctx context.Context) ([]string, error) {
	return p.packageNames(ctx, "-Qqe")
}

func (p Probe) packageNames(ctx context.Context, flags string) ([]string, error) {
	out, err := p.output(ctx, "pacman", flags)
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// LoginShell returns field 7 of the user's passwd entry.
func (p Probe) LoginShell(ctx context.Context) (string, error) {
	out, err := p.output(ctx, "getent", "passwd", p.User)
	if err != nil {
		return "", err
	}
	fields := strings.Split(strings.TrimSpace(out), ":")
	if len(fields) <= passwdShellField {
		return "", fmt.Errorf("getent passwd %s: malformed entry", p.User)
	}
	return fields[passwdShellField], nil
}

func (Probe) HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (Probe) IsRoot() bool { return os.Geteuid() == 0 }

// ReadSystemFile reads a file from the allow-list only.
func (Probe) ReadSystemFile(p string) ([]byte, error) {
	if !IsAllowedSystemPath(p) {
		return nil, fmt.Errorf("%w: %s", ErrPathNotAllowed, p)
	}
	return os.ReadFile(p)
}

// IsAllowedSystemPath reports whether p is /etc/pacman.conf or a direct child of the logind drop-in dir.
// The check is lexical by design: symlinks are not resolved.
func IsAllowedSystemPath(p string) bool {
	if path.Clean(p) != p {
		return false
	}
	return p == pacmanConfPath || path.Dir(p) == logindDropInDir
}
