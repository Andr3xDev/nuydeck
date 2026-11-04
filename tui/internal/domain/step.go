package domain

// Step is a closed sum of every write the tool can perform; the marker
// method keeps implementations inside this package.
type Step interface {
	isStep()
}

// CopyStep runs rsync with --files-from for the selected paths of one entry.
type CopyStep struct {
	Entry     ResolvedEntry
	Direction Direction
	Src, Dst  string
	Paths     []string
	Dir       bool
}

// DeleteStep removes a single path.
type DeleteStep struct {
	Path string
	Root string // destination root the path must stay inside
}

// BackupStep copies files that are about to be overwritten or deleted.
type BackupStep struct {
	Direction Direction
	Files     []BackupFile
	Roots     map[Root]string
}

type BackupFile struct {
	Root Root
	Path string
}

type DeleteBackupStep struct {
	Dir string
}

type SaveStateStep struct {
	State State
}

type AppendLogStep struct {
	Entry LogEntry
}

func (CopyStep) isStep()         {}
func (DeleteStep) isStep()       {}
func (BackupStep) isStep()       {}
func (DeleteBackupStep) isStep() {}
func (SaveStateStep) isStep()    {}
func (AppendLogStep) isStep()    {}

type StepStatus int

const (
	StepOK StepStatus = iota
	StepFailed
	StepWouldApply
)

type StepResult struct {
	Status StepStatus
	Detail string
	Err    error
	// Applied lists the items done before a failure.
	Applied []string
}

// StepProgress reports one transferred item.
type StepProgress struct {
	Path string
}

// LogEntry is one JSONL record of a write operation.
type LogEntry struct {
	Time      string    `json:"ts"`
	Op        string    `json:"op"`
	Direction string    `json:"direction,omitempty"`
	DryRun    bool      `json:"dry_run"`
	Items     []LogItem `json:"items,omitempty"`
	Result    string    `json:"result"`
	Error     string    `json:"error,omitempty"`
}

type LogItem struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Result string `json:"result"`
}
