package state

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
