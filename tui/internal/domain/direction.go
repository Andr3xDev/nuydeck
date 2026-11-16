package domain

type Direction int

const (
	Push Direction = iota
	Pull
	Restore
)
