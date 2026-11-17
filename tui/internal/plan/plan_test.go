package plan

import "testing"

func TestPlanEmpty(t *testing.T) {
	if !(Plan{}).Empty() {
		t.Error("a plan without steps must be empty")
	}
	if (Plan{Steps: []Step{CommandStep{Label: "x"}}}).Empty() {
		t.Error("a plan with a real step must not be empty")
	}
}
