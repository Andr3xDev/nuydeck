package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	secretsFileMode     fs.FileMode = 0o600
	rejectedSecretChars             = "\r\n\x00"
)

// writeSecrets never reports a value: errors carry the path, the variable name or the errno only.
func writeSecrets(s plan.WriteSecretsStep) plan.StepResult {
	names := make([]string, len(s.Entries))
	for i, e := range s.Entries {
		names[i] = e.Name
	}
	if err := manifest.ValidateSecretNames(names); err != nil {
		return failed(err)
	}
	var body strings.Builder
	for _, e := range s.Entries {
		value := e.Value.Reveal()
		if strings.ContainsAny(value, rejectedSecretChars) {
			return failed(fmt.Errorf("secret %s contains a line break or NUL", e.Name))
		}
		fmt.Fprintf(&body, "export %s='%s'\n", e.Name, strings.ReplaceAll(value, "'", `'\''`))
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), userDirMode); err != nil {
		return failed(err)
	}
	f, err := os.OpenFile(s.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, secretsFileMode)
	if err != nil {
		return failed(fmt.Errorf("create %s: %w", s.Path, errnoOf(err)))
	}
	// The creation mode is filtered by the umask; Fchmod fixes it before any value is written.
	err = f.Chmod(secretsFileMode)
	if err == nil {
		_, err = f.WriteString(body.String())
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(s.Path) // our own partial file would otherwise block the retry
		return failed(fmt.Errorf("write %s: %w", s.Path, errnoOf(err)))
	}
	return plan.StepResult{Status: plan.StepOK}
}

func errnoOf(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}
