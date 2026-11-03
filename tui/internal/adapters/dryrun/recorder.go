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
		return "would save state"
	case domain.AppendLogStep:
		return "would append log entry: " + s.Entry.Op
	default:
		return fmt.Sprintf("would run %T", s)
	}
}
