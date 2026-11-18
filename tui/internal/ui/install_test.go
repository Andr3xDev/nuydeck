package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const waitTimeout = 2 * time.Second

type fakeInstaller struct {
	plan    install.InstallPlan
	runs    atomic.Int32
	run     func(ctx context.Context, events chan<- plan.Event, ack <-chan bool) install.InstallResult
	drafted []install.Draft
}

func (f *fakeInstaller) Plan(_ context.Context, d install.Draft) (install.InstallPlan, error) {
	f.drafted = append(f.drafted, d)
	return f.plan, nil
}

func (f *fakeInstaller) Run(ctx context.Context, _ install.InstallPlan, events chan<- plan.Event, ack <-chan bool) install.InstallResult {
	f.runs.Add(1)
	defer close(events)
	if f.run == nil {
		return install.InstallResult{}
	}
	return f.run(ctx, events, ack)
}

func onePlan() install.InstallPlan {
	return install.InstallPlan{Steps: []install.PlannedStep{{Phase: install.PhaseReboot, Step: plan.CommandStep{Label: "reboot"}}}}
}

func newTestRoot(f *fakeInstaller, secretNames ...string) (*Root, *Install) {
	root := NewRoot("", false, Deps{Install: InstallDeps{
		Plan:        f.Plan,
		Run:         f.Run,
		Drivers:     []manifest.Driver{{ID: "amd", VendorID: "1002"}, {ID: "nvidia", VendorID: "10de"}},
		SecretNames: secretNames,
	}})
	in, _, _ := sectionOf[*Install](root)
	return root, in
}

func specialKey(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

// settle feeds command results back until no command is left; only for commands that return at once.
func settle(in *Install, cmd tea.Cmd) {
	for cmd != nil {
		_, cmd = in.Update(cmd())
	}
}

// tap also renders the page, so every page state is exercised for panics.
func tap(in *Install, k tea.KeyPressMsg) tea.Cmd {
	_, cmd := in.Update(k)
	in.View(100, 30)
	return cmd
}

// toHardware answers the profile page with the default minimal profile.
func toHardware(in *Install) {
	settle(in, tap(in, specialKey(tea.KeyEnter)))
}

func toSummary(in *Install) {
	toHardware(in)
	settle(in, tap(in, specialKey(tea.KeyEnter)))
}

func TestInstallBackKeepsDraftIncludingSkipGPUEnvLink(t *testing.T) {
	f := &fakeInstaller{plan: onePlan()}
	_, in := newTestRoot(f)
	toHardware(in)
	in.Update(specialKey(tea.KeyDown))
	in.Update(specialKey(tea.KeyDown))
	in.Update(keyPress(tea.KeySpace))
	if !in.draft.SkipGPUEnvLink {
		t.Fatal("space on the skip row did not set SkipGPUEnvLink")
	}

	settle(in, tap(in, specialKey(tea.KeyEnter)))
	if in.page != pageSummary {
		t.Fatalf("page = %v, want the summary", in.page)
	}
	tap(in, specialKey(tea.KeyEscape))

	if in.page != pageHardware {
		t.Fatalf("page after back = %v, want hardware", in.page)
	}
	if !in.draft.SkipGPUEnvLink || len(in.draft.Drivers) != 2 {
		t.Errorf("draft after back = %+v, want SkipGPUEnvLink and both drivers kept", in.draft)
	}
	if !f.drafted[0].SkipGPUEnvLink {
		t.Error("the plan was built without the skip choice")
	}
}

func TestInstallRunsNothingBeforeExplicitConfirm(t *testing.T) {
	t.Run("confirm hands the run to the shell", func(t *testing.T) {
		f := &fakeInstaller{plan: onePlan()}
		root, in := newTestRoot(f)
		toSummary(in)

		for _, k := range []tea.KeyPressMsg{keyPress('x'), specialKey(tea.KeyDown), keyPress(tea.KeySpace)} {
			tap(in, k)
		}
		if in.page != pageSummary || f.runs.Load() != 0 {
			t.Fatalf("before confirm: page=%v runs=%d, want the summary and no run", in.page, f.runs.Load())
		}

		cmd := tap(in, specialKey(tea.KeyEnter))
		if !isRunRequest(cmd) || f.runs.Load() != 0 {
			t.Fatalf("confirm: run request=%v runs=%d, want a request and nothing run yet", isRunRequest(cmd), f.runs.Load())
		}
		if again := tap(in, specialKey(tea.KeyEnter)); again != nil {
			t.Error("a second confirm must not ask for another run")
		}
		d := newDriver(t, root)
		d.launch(cmd)
		d.must(t, "the install to run", func() bool { return f.runs.Load() == 1 })
	})

	t.Run("blocking errors offer no confirm", func(t *testing.T) {
		plan := onePlan()
		plan.Blocking = []error{errors.New("pacman.conf has no multilib section")}
		f := &fakeInstaller{plan: plan}
		_, in := newTestRoot(f)
		toSummary(in)

		if cmd := tap(in, specialKey(tea.KeyEnter)); cmd != nil || in.page != pageSummary || f.runs.Load() != 0 {
			t.Errorf("confirm with blocking errors: page=%v runs=%d, want to stay on the summary", in.page, f.runs.Load())
		}
	})
}

// awaitingAck drives a run to the reboot acknowledgement; the fake reports how Run ended.
func awaitingAck(t *testing.T) (d *driver, acked, cancelled chan struct{}) {
	t.Helper()
	acked, cancelled = make(chan struct{}, 1), make(chan struct{}, 1)
	f := &fakeInstaller{plan: onePlan(), run: func(ctx context.Context, events chan<- plan.Event, ack <-chan bool) install.InstallResult {
		events <- plan.Event{Index: 0, Kind: plan.EventAwaitAck}
		select {
		case <-ack:
			acked <- struct{}{}
		case <-ctx.Done():
			cancelled <- struct{}{}
		}
		return install.InstallResult{}
	}}
	root, in := newTestRoot(f)
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	toSummary(in)
	d = newDriver(t, root)
	d.launch(tap(in, specialKey(tea.KeyEnter)))
	d.must(t, "the reboot prompt", func() bool { return root.run != nil && root.run.awaitAck })
	if view := d.view(); !strings.Contains(view, "Press Enter to reboot") || !strings.Contains(view, "[>] reboot") {
		t.Errorf("want the reboot prompt and the running step mark:\n%s", view)
	}
	return d, acked, cancelled
}

func receives(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	case <-time.After(waitTimeout):
		return false
	}
}

func TestInstallRebootAckAcceptsEnterOnly(t *testing.T) {
	t.Run("other keys do nothing, enter acknowledges", func(t *testing.T) {
		d, acked, cancelled := awaitingAck(t)

		for _, k := range []tea.KeyPressMsg{keyPress('x'), keyPress('y'), keyPress(tea.KeySpace), specialKey(tea.KeyEscape), keyPress('q')} {
			d.press(k)
		}
		d.settle()
		if d.quit || !d.root.run.awaitAck || len(d.root.run.ack) != 0 {
			t.Fatalf("after other keys: quit=%v awaitAck=%v pendingAck=%d, want the prompt untouched", d.quit, d.root.run.awaitAck, len(d.root.run.ack))
		}

		d.press(specialKey(tea.KeyEnter))
		if !receives(acked) {
			t.Fatal("enter did not reach Run")
		}
		if len(cancelled) != 0 {
			t.Error("the run was cancelled")
		}
	})

	t.Run("ctrl+c cancels and quits", func(t *testing.T) {
		d, acked, cancelled := awaitingAck(t)

		d.press(keyCtrlC)

		d.must(t, "the program to quit", func() bool { return d.quit })
		if !receives(cancelled) {
			t.Error("ctrl+c did not cancel the run")
		}
		if len(acked) != 0 {
			t.Error("ctrl+c acknowledged the reboot")
		}
	})
}

func TestInstallForgetsSecretValuesAfterTheRun(t *testing.T) {
	const sentinel = "hunter2-sentinel"
	plan := install.InstallPlan{Steps: []install.PlannedStep{{Phase: install.PhaseEnv, Step: plan.WriteSecretsStep{
		Path: "/home/u/.secrets", Entries: []plan.SecretEntry{{Name: "TOKEN", Value: sentinel}},
	}}}}
	f := &fakeInstaller{plan: plan}
	root, in := newTestRoot(f, "TOKEN")
	toHardware(in)
	settle(in, tap(in, specialKey(tea.KeyEnter)))
	for _, r := range sentinel {
		tap(in, keyPress(r))
	}
	settle(in, tap(in, specialKey(tea.KeyEnter)))
	if in.page != pageSummary {
		t.Fatalf("page = %v, want the summary", in.page)
	}
	if w := f.drafted[0].Secrets; len(w) != 1 || w[0].Value.Reveal() != sentinel {
		t.Fatalf("plan draft secrets = %v, want the typed value", w)
	}

	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	d := newDriver(t, root)
	d.launch(tap(in, specialKey(tea.KeyEnter)))
	d.must(t, "the run to finish", d.runDone)

	dump := fmt.Sprintf("%#v", in.plan) + in.View(100, 30) + d.view()
	if strings.Contains(dump, sentinel) {
		t.Errorf("secret value in the plan or a view after the run: %s", dump)
	}
	if w := in.draft.Secrets; len(w) != 1 || w[0].Value.Reveal() != sentinel {
		t.Errorf("draft secrets = %v, want the value kept for a retry", w)
	}
	in.Leave()
	if len(in.draft.Secrets) != 0 {
		t.Errorf("draft secrets after Leave = %v, want none", in.draft.Secrets)
	}
}

func TestInstallSecretsPage(t *testing.T) {
	cases := []struct {
		name        string
		names       []string
		keys        []tea.KeyPressMsg
		wantPage    installPage
		wantEntries []string
		wantSkipped bool
	}{
		{name: "no names passes through", wantPage: pageSummary},
		{name: "empty value omits", names: []string{"TOKEN"}, keys: []tea.KeyPressMsg{specialKey(tea.KeyEnter)}, wantPage: pageSummary},
		{name: "typed value is kept", names: []string{"TOKEN"}, keys: []tea.KeyPressMsg{keyPress('a'), keyPress('b'), specialKey(tea.KeyEnter)},
			wantPage: pageSummary, wantEntries: []string{"TOKEN=ab"}},
		{name: "ctrl+s skips", names: []string{"TOKEN"}, keys: []tea.KeyPressMsg{{Code: 's', Mod: tea.ModCtrl}}, wantPage: pageSummary, wantSkipped: true},
		{name: "a value starting with s is typed", names: []string{"TOKEN"}, keys: []tea.KeyPressMsg{keyPress('s'), keyPress('e'), specialKey(tea.KeyEnter)},
			wantPage: pageSummary, wantEntries: []string{"TOKEN=se"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, in := newTestRoot(&fakeInstaller{plan: onePlan()}, c.names...)
			toHardware(in)
			settle(in, tap(in, specialKey(tea.KeyEnter)))
			if len(c.names) > 0 && in.page != pageSecrets {
				t.Fatalf("page = %v, want the secrets page", in.page)
			}
			for _, k := range c.keys {
				settle(in, tap(in, k))
			}

			var got []string
			for _, e := range in.draft.Secrets {
				got = append(got, e.Name+"="+e.Value.Reveal())
			}
			if in.page != c.wantPage || in.draft.SkipSecrets != c.wantSkipped || len(got) != len(c.wantEntries) || (len(got) > 0 && got[0] != c.wantEntries[0]) {
				t.Errorf("page=%v skipped=%v entries=%v, want page=%v skipped=%v entries=%v",
					in.page, in.draft.SkipSecrets, got, c.wantPage, c.wantSkipped, c.wantEntries)
			}
		})
	}
}
