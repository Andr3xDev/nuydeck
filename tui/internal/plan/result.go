package plan

type StepStatus int

const (
	StepOK StepStatus = iota
	StepFailed
	StepWouldApply
)

// KeptNotEmpty is the detail of a delete that left a non-empty directory in place.
const KeptNotEmpty = "kept (not empty)"

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
	Line string
}
