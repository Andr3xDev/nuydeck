package domain

import (
	"io/fs"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

// Step is a closed sum of every write the tool can perform; the marker
// method keeps implementations inside this package.
type Step interface {
	isStep()
}

// CopyStep runs rsync with --files-from for the selected paths of one entry.
type CopyStep struct {
	Entry     manifest.ResolvedEntry
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
	Roots     map[manifest.Root]string
}

type BackupFile struct {
	Root manifest.Root
	Path string
}

type DeleteBackupStep struct {
	Dir string
}

type SaveStateStep struct {
	State state.State
}

type AppendLogStep struct {
	Entry state.LogEntry
}

// CommandStep runs an external command.
type CommandStep struct {
	Label string
	Argv  []string
	Dir   string
	Sudo  bool
	// UsesSudo marks a command that calls sudo itself, so the Runner must not prefix it but the run still guards its credentials.
	UsesSudo    bool
	Interactive bool
	Quiet       bool
}

// WriteFileStep writes a file, optionally through sudo, keeping the previous copy in BackupTo.
type WriteFileStep struct {
	Path       string
	Content    []byte
	Mode       fs.FileMode
	Privileged bool
	BackupTo   string
}

type SymlinkStep struct {
	Path   string
	Target string
}

// WriteSecretsStep creates the secrets file from variable names; values stay redacted outside Reveal.
type WriteSecretsStep struct {
	Path    string
	Entries []SecretEntry
}

type SecretEntry struct {
	Name string
	// go-toml encodes string-kind types raw and ignores TextMarshaler, so the tag is the only guard there.
	Value Secret `toml:"-"`
}

// AppendPackageStep adds one package block to the catalog file.
type AppendPackageStep struct {
	Path  string
	Entry manifest.Package
}

func (CopyStep) isStep()          {}
func (DeleteStep) isStep()        {}
func (BackupStep) isStep()        {}
func (DeleteBackupStep) isStep()  {}
func (SaveStateStep) isStep()     {}
func (AppendLogStep) isStep()     {}
func (CommandStep) isStep()       {}
func (WriteFileStep) isStep()     {}
func (SymlinkStep) isStep()       {}
func (WriteSecretsStep) isStep()  {}
func (AppendPackageStep) isStep() {}
