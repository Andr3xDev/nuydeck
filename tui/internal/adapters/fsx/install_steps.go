package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

const (
	secretsFileMode     fs.FileMode = 0o600
	userDirMode         fs.FileMode = 0o755
	keptNotSymlink                  = "kept (not a symlink)"
	rejectedSecretChars             = "\r\n\x00"
)

// WriteAtomic writes data to a temp file beside path, sets mode and renames it over path.
func WriteAtomic(path string, data []byte, mode fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), mode)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}

func writeFile(s domain.WriteFileStep) domain.StepResult {
	if s.Privileged {
		return failed(fmt.Errorf("privileged write of %s needs the system runner", s.Path))
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), userDirMode); err != nil {
		return failed(err)
	}
	return done(WriteAtomic(s.Path, s.Content, s.Mode))
}

func symlink(s domain.SymlinkStep) domain.StepResult {
	if err := os.MkdirAll(filepath.Dir(s.Path), userDirMode); err != nil {
		return failed(err)
	}
	if info, err := os.Lstat(s.Path); err == nil && info.Mode()&fs.ModeSymlink == 0 {
		return domain.StepResult{Status: domain.StepOK, Detail: keptNotSymlink}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return failed(err)
	}
	tmp := s.Path + ".nuydeck-tmp"
	_ = os.Remove(tmp) // stale leftover of an interrupted run
	if err := os.Symlink(s.Target, tmp); err != nil {
		return failed(err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		_ = os.Remove(tmp)
		return failed(err)
	}
	return domain.StepResult{Status: domain.StepOK}
}

// writeSecrets never reports a value: errors carry the path, the variable name or the errno only.
func writeSecrets(s domain.WriteSecretsStep) domain.StepResult {
	names := make([]string, len(s.Entries))
	for i, e := range s.Entries {
		names[i] = e.Name
	}
	if err := domain.ValidateSecretNames(names); err != nil {
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
	return domain.StepResult{Status: domain.StepOK}
}

func errnoOf(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

func done(err error) domain.StepResult {
	if err != nil {
		return failed(err)
	}
	return domain.StepResult{Status: domain.StepOK}
}
