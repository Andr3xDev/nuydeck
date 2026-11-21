package plan

type Direction int

const (
	Push Direction = iota
	Pull
	Restore
	Manual
)
