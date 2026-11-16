package app

type InstallEventKind int

const (
	EventStarted InstallEventKind = iota
	EventLine
	EventDone
	EventSkipped
	EventFailed
	EventAwaitAck
)

// InstallEvent reports progress of the step at Index in InstallPlan.Steps.
type InstallEvent struct {
	Index int
	Kind  InstallEventKind
	Line  string
	Err   error
}
