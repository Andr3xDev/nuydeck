package dryrun

import (
	"context"
	"fmt"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// Recorder is an app.Runner that renders what each Step would do and performs no I/O.
type Recorder struct{}

func (Recorder) Run(_ context.Context, s domain.Step, _ func(domain.StepProgress)) domain.StepResult {
	return domain.StepResult{Status: domain.StepWouldApply, Detail: render(s)}
}

func render(s domain.Step) string {
	switch s := s.(type) {
	case domain.CopyStep:
		return fmt.Sprintf("would copy %s -> %s: %s", s.Src, s.Dst, strings.Join(s.Paths, ", "))
	case domain.DeleteStep:
		return "would delete " + s.Path
	case domain.BackupStep:
		paths := make([]string, len(s.Files))
		for i, f := range s.Files {
			paths[i] = string(f.Root) + "/" + f.Path
		}
		return "would back up " + strings.Join(paths, ", ")
	case domain.DeleteBackupStep:
		return "would delete backup " + s.Dir
	case domain.SaveStateStep:
		if len(s.State.IgnoredPackages) == 0 {
			return "would save state"
		}
		return "would save state; ignored_packages: " + strings.Join(s.State.IgnoredPackages, ", ")
	case domain.CommandStep:
		argv := s.Argv
		if s.Sudo {
			argv = append([]string{"sudo"}, argv...)
		}
		return "would run " + strings.Join(argv, " ")
	case domain.WriteFileStep:
		detail := fmt.Sprintf("would write %s (%d bytes)", s.Path, len(s.Content))
		if s.Privileged {
			detail += " with sudo"
		}
		if s.BackupTo != "" {
			detail += ", backup to " + s.BackupTo
		}
		return detail
	case domain.SymlinkStep:
		return fmt.Sprintf("would link %s -> %s", s.Path, s.Target)
	case domain.WriteSecretsStep:
		if len(s.Entries) == 0 {
			return fmt.Sprintf("would create empty %s (mode 600)", s.Path)
		}
		names := make([]string, len(s.Entries))
		for i, e := range s.Entries {
			names[i] = e.Name
		}
		return fmt.Sprintf("would create %s with variables: %s", s.Path, strings.Join(names, ", "))
	case domain.AppendPackageStep:
		return fmt.Sprintf("would append package %s (%s) to %s", s.Entry.Name, s.Entry.Source, s.Path)
	case domain.AppendLogStep:
		return "would append log entry: " + s.Entry.Op
	default:
		return fmt.Sprintf("would run %T", s)
	}
}
