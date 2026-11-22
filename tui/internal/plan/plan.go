package plan

// Plan is what a flow previews, confirms and applies.
type Plan struct {
	Title   string
	Steps   []Step
	Notices []string
	// Blocking errors forbid running the plan.
	Blocking []error
	// Confirm is the text of the normal confirm; the flow supplies a default when empty.
	Confirm string
	// ExtraConfirm is an optional second confirm; empty means none.
	ExtraConfirm string
}

// Empty reports whether the plan has no step beyond the always-run markers.
func (p Plan) Empty() bool {
	for _, s := range p.Steps {
		if _, always := s.(Always); !always {
			return false
		}
	}
	return true
}
