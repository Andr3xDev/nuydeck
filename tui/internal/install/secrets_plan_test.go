package install

import (
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestPlan_SecretsStepExistsOnlyWhenTheDraftCarriesAValue(t *testing.T) {
	cases := []struct {
		name      string
		secrets   []plan.SecretEntry
		skip      bool
		wantEntry string
	}{
		{name: "a typed value keeps the step", secrets: []plan.SecretEntry{{Name: "API_KEY", Value: "k1"}}, wantEntry: "API_KEY"},
		{name: "re-plan with the same secrets keeps the step", secrets: []plan.SecretEntry{{Name: "API_KEY", Value: "k1"}, {Name: "OTHER", Value: "k2"}}, wantEntry: "API_KEY"},
		{name: "no secrets yields no step"},
		{name: "only empty values yield no step", secrets: []plan.SecretEntry{{Name: "EMPTY", Value: ""}}},
		{name: "skip yields no step", secrets: []plan.SecretEntry{{Name: "API_KEY", Value: "k1"}}, skip: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlanFixture(t)
			f.draft.Secrets, f.draft.SkipSecrets = tc.secrets, tc.skip

			steps := stepsOfType[plan.WriteSecretsStep](rawSteps(f.plan(t).Steps))

			if tc.wantEntry == "" {
				if len(steps) != 0 {
					t.Fatalf("secrets steps = %+v, want none", steps)
				}
				return
			}
			if len(steps) != 1 || len(steps[0].Entries) == 0 || steps[0].Entries[0].Name != tc.wantEntry {
				t.Fatalf("secrets steps = %+v, want one step starting with %q", steps, tc.wantEntry)
			}
		})
	}
}
