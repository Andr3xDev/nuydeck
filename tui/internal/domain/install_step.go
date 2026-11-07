package domain

import "io/fs"

// Phase is a stage of the guided install; the numeric order is the execution order.
type Phase int

const (
	PhasePrivilege Phase = iota
	PhasePacmanConf
	PhasePacman
	PhaseParuBootstrap
	PhaseAur
	PhaseSystem
	PhaseConfigs
	PhaseFonts
	PhaseEnv
	PhaseShell
	PhaseServices
	PhaseTheme
	PhaseRecord
	PhaseReboot
)

// PlannedStep is a step tagged with its phase; a non-empty Skip is the reason it will not run.
type PlannedStep struct {
	Phase Phase
	Step  Step
	Skip  string
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
	Entry Package
}

func (CommandStep) isStep()       {}
func (WriteFileStep) isStep()     {}
func (SymlinkStep) isStep()       {}
func (WriteSecretsStep) isStep()  {}
func (AppendPackageStep) isStep() {}

const redacted = "[redacted]"

// Secret hides its value from fmt, JSON and TOML encoders; Reveal is the only reader.
// fmt skips Stringer/GoString/TextMarshaler for values behind unexported struct fields and string(s) skips
// redaction, so keep a Secret in exported fields of exported types only.
type Secret string

func (Secret) String() string               { return redacted }
func (Secret) GoString() string             { return redacted }
func (Secret) MarshalText() ([]byte, error) { return []byte(redacted), nil }
func (s Secret) Reveal() string             { return string(s) }
