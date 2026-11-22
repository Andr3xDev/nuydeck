package plan

type EventKind int

const (
	EventStarted EventKind = iota
	EventLine
	EventDone
	EventSkipped
	EventFailed
	EventAwaitAck
)

// Event reports progress of the step at Index of Total.
type Event struct {
	Index, Total int
	Label        string
	Kind         EventKind
	Line         string
	Err          error
	YesNo        bool // with EventAwaitAck: Line is a yes/no question, not a plain acknowledgement
}
