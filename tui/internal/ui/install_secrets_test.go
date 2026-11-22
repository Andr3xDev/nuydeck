package ui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const secretValue = "hunter2-sentinel"

// toSummaryWithSecret types one secret and stops on the summary.
func toSummaryWithSecret(t *testing.T, in *Install) {
	t.Helper()
	toHardware(in)
	settle(in, tap(in, specialKey(tea.KeyEnter)))
	for _, r := range secretValue {
		tap(in, keyPress(r))
	}
	settle(in, tap(in, specialKey(tea.KeyEnter)))
	if in.page != pageSummary {
		t.Fatalf("page = %v, want the summary", in.page)
	}
}

// replan plans again with whatever the wizard holds and returns that draft.
func replan(in *Install, f *fakeInstaller) install.Draft {
	settle(in, in.startPlan())
	return f.drafted[len(f.drafted)-1]
}

func secretValues(d install.Draft) []string {
	var got []string
	for _, e := range d.Secrets {
		got = append(got, e.Value.Reveal())
	}
	return got
}

func TestInstallRetryReplansWithTheTypedSecrets(t *testing.T) {
	// A successful run resets the wizard, so only a failed one leaves a summary to retry from.
	f := &fakeInstaller{plan: onePlan(), run: func(context.Context, chan<- plan.Event, <-chan bool) install.InstallResult {
		return install.InstallResult{Err: errors.New("step failed")}
	}}
	root, in := newTestRoot(f, "TOKEN")
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	toSummaryWithSecret(t, in)

	d := newDriver(t, root)
	d.launch(tap(in, specialKey(tea.KeyEnter)))
	d.must(t, "the run to finish", d.runDone)
	d.press(specialKey(tea.KeyEscape))
	d.must(t, "the retry plan", func() bool { return !in.planning && len(f.drafted) == 2 })

	if got := secretValues(f.drafted[1]); len(got) != 1 || got[0] != secretValue {
		t.Errorf("retry plan secrets = %v, want the typed value without retyping", got)
	}
}

func TestInstallForgetsSecretsWhenTheOwnerLeavesOrQuits(t *testing.T) {
	cases := map[string]func(root *Root){
		"selection moves off Install": func(root *Root) { root.selectSection(0) },
		"quit":                        func(root *Root) { root.quit() },
	}
	for name, leave := range cases {
		t.Run(name, func(t *testing.T) {
			f := &fakeInstaller{plan: onePlan()}
			root, in := newTestRoot(f, "TOKEN")
			_, idx, _ := sectionOf[*Install](root)
			root.selectSection(idx)
			toSummaryWithSecret(t, in)
			if got := secretValues(replan(in, f)); len(got) != 1 {
				t.Fatalf("before leaving, plan secrets = %v, want the typed value", got)
			}

			leave(root)

			if got := secretValues(replan(in, f)); len(got) != 0 {
				t.Errorf("after leaving, plan secrets = %v, want none", got)
			}
		})
	}
}
