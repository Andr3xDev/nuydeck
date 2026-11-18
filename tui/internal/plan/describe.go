package plan

import "fmt"

// Describe names a step for the log and the UI without argv or secret values.
func Describe(s Step) (label, kind string) {
	switch s := s.(type) {
	case CommandStep:
		return s.Label, "command"
	case ServiceActionStep:
		return fmt.Sprintf("%s %s (%s)", s.Action, s.Unit, s.Scope), "service"
	case ConfirmStep:
		return s.Prompt, "confirm"
	case WriteFileStep:
		return s.Path, "file"
	case SymlinkStep:
		return s.Path, "link"
	case WriteSecretsStep:
		return s.Path, "secrets"
	case CopyStep:
		return string(s.Entry.Root) + ":" + s.Entry.Path, "copy"
	case BackupStep:
		return "back up overwritten files", "backup"
	case DeleteStep:
		return s.Path, "delete"
	case DeleteBackupStep:
		return s.Dir, "delete-backup"
	case SaveStateStep:
		return "save state", "state"
	case AppendLogStep:
		return "record install log", "log"
	case AppendPackageStep:
		return s.Path, "package"
	}
	return fmt.Sprintf("%T", s), "step"
}
