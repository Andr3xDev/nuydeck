package plan

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
	Line string
}
