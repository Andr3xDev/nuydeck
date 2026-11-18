package install

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const (
	runTimeout    = 3 * time.Second
	eventBuffer   = 256
	keepaliveTick = 5 * time.Millisecond
	settle        = 30 * time.Millisecond

	handoffLabel = "sudo password"
	probeLabel   = "verify sudo"

	secretSentinel = "hunter2-sentinel"
)

func newRunInstaller(r plan.Runner) *Installer {
	return &Installer{Runner: r, Preview: r, Now: func() time.Time { return testNow }, KeepaliveEvery: time.Hour}
}

func planned(phase Phase, s plan.Step) PlannedStep {
	return PlannedStep{Phase: phase, Step: s}
}

func command(label string) PlannedStep {
	return planned(PhasePacman, plan.CommandStep{Label: label, Argv: []string{label}})
}

func sudoHandoff() PlannedStep {
	return planned(PhasePrivilege, plan.CommandStep{Label: handoffLabel, Argv: []string{"sudo", "-v"}, Interactive: true})
}

func sudoProbe() PlannedStep {
	return planned(PhasePrivilege, plan.CommandStep{Label: probeLabel, Argv: []string{"sudo", "-n", "-v"}, Quiet: true})
}

func recordStep() PlannedStep {
	return planned(PhaseRecord, plan.AppendLogStep{Entry: state.LogEntry{Op: "install"}})
}

func rebootStep() PlannedStep {
	return planned(PhaseReboot, plan.CommandStep{Label: "reboot", Argv: []string{"systemctl", "reboot"}, Sudo: true})
}

func isSudoProbe(s plan.Step) bool {
	c, ok := s.(plan.CommandStep)
	return ok && c.Quiet && slices.Equal(c.Argv, []string{"sudo", "-n", "-v"})
}

func labelOf(s plan.Step) string {
	c, _ := s.(plan.CommandStep)
	return c.Label
}

func runInstallCtx(t *testing.T, ctx context.Context, inst *Installer, steps []PlannedStep, ack <-chan bool) (InstallResult, []plan.Event) {
	t.Helper()
	events := make(chan plan.Event, eventBuffer)
	done := make(chan InstallResult, 1)
	go func() { done <- inst.Run(ctx, InstallPlan{Steps: steps}, events, ack) }()
	select {
	case res := <-done:
		var collected []plan.Event
		for {
			select {
			case e, ok := <-events:
				if !ok {
					return res, collected
				}
				collected = append(collected, e)
			default:
				return res, collected
			}
		}
	case <-time.After(runTimeout):
		t.Fatal("Run did not return")
		return InstallResult{}, nil
	}
}

func runInstall(t *testing.T, inst *Installer, steps []PlannedStep, ack <-chan bool) (InstallResult, []plan.Event) {
	t.Helper()
	return runInstallCtx(t, context.Background(), inst, steps, ack)
}

var eventNames = map[plan.EventKind]string{
	plan.EventStarted: "started", plan.EventLine: "line", plan.EventDone: "done",
	plan.EventSkipped: "skipped", plan.EventFailed: "failed", plan.EventAwaitAck: "await-ack",
}

func describe(events []plan.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = fmt.Sprintf("%s:%d", eventNames[e.Kind], e.Index)
		if e.Kind == plan.EventLine {
			out[i] += ":" + e.Line
		}
	}
	return out
}

func hasEvent(events []plan.Event, kind plan.EventKind, index int) bool {
	return slices.ContainsFunc(events, func(e plan.Event) bool { return e.Kind == kind && e.Index == index })
}

// waitUntil reports a timeout with t.Errorf because it is also called from the runner goroutine, where FailNow is not allowed.
func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(runTimeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Error("condition not reached in time")
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func awaitEvent(t *testing.T, events <-chan plan.Event, kind plan.EventKind) {
	t.Helper()
	timeout := time.After(runTimeout)
	for {
		select {
		case e := <-events:
			if e.Kind == kind {
				return
			}
		case <-timeout:
			t.Fatalf("no %s event", eventNames[kind])
		}
	}
}

func TestRun_EmitsEventsInOrderAndSkipsWithoutCallingTheRunner(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
		if labelOf(s) == "first" {
			progress(plan.StepProgress{Line: "hello"})
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	steps := []PlannedStep{
		command("first"),
		{Phase: PhasePacman, Step: plan.CommandStep{Label: "skipped", Argv: []string{"skipped"}}, Skip: "already installed"},
		command("third"),
	}

	res, events := runInstall(t, newRunInstaller(r), steps, nil)

	want := []string{"started:0", "line:0:hello", "done:0", "skipped:1", "started:2", "done:2"}
	if got := describe(events); !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
	if got := r.commandLabels(); !slices.Equal(got, []string{"first", "third"}) {
		t.Errorf("runner received %v, want first and third only", got)
	}
	if res.Err != nil {
		t.Errorf("Err = %v", res.Err)
	}
}

func TestRun_FailureStopsLaterStepsRecordsTheFailureAndNeverReboots(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		if labelOf(s) == "two" {
			return failedStep("pacman exited 1")
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	ack := make(chan bool, 1)
	ack <- true

	res, events := runInstall(t, newRunInstaller(r), []PlannedStep{
		command("one"), command("two"), command("three"), recordStep(), rebootStep(),
	}, ack)

	if got := r.commandLabels(); !slices.Equal(got, []string{"one", "two"}) {
		t.Errorf("commands run = %v, want one and two only", got)
	}
	logs := stepsOfType[plan.AppendLogStep](r.ran())
	if len(logs) != 1 || logs[0].Entry.Op != "install" || logs[0].Entry.Result != "failed" {
		t.Errorf("log record = %+v, want one install entry with result failed", logs)
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "pacman exited 1") {
		t.Errorf("Err = %v, want the command error", res.Err)
	}
	if !hasEvent(events, plan.EventFailed, 1) || hasEvent(events, plan.EventAwaitAck, 4) {
		t.Errorf("events = %v, want failed:1 and no await-ack", describe(events))
	}
}

func TestRun_WrongPasswordStopsBeforeAnyStep(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		if c, ok := s.(plan.CommandStep); ok && c.Interactive {
			return failedStep("sudo: exit status 1")
		}
		return plan.StepResult{Status: plan.StepOK}
	}

	res, _ := runInstall(t, newRunInstaller(r), []PlannedStep{
		sudoHandoff(), sudoProbe(), command("pacman"), recordStep(), rebootStep(),
	}, nil)

	if got := r.commandLabels(); !slices.Equal(got, []string{handoffLabel}) {
		t.Errorf("commands run = %v, want only the handoff", got)
	}
	if res.Err == nil {
		t.Error("Err is nil after a failed handoff")
	}
}

func TestRun_FailedVerificationProbeStopsBeforeTheFirstMutatingStep(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		if isSudoProbe(s) {
			return failedStep("exit status 1")
		}
		return plan.StepResult{Status: plan.StepOK}
	}

	res, events := runInstall(t, newRunInstaller(r), []PlannedStep{
		sudoHandoff(), sudoProbe(), command("pacman"), recordStep(), rebootStep(),
	}, nil)

	if got := r.commandLabels(); !slices.Equal(got, []string{handoffLabel, probeLabel}) {
		t.Errorf("commands run = %v, want the handoff and the probe only", got)
	}
	const want = "sudo credentials are not visible to child processes (sudo timestamp_type); see the install log"
	if res.Err == nil || !strings.Contains(res.Err.Error(), want) {
		t.Errorf("Err = %v, want message %q", res.Err, want)
	}
	if !hasEvent(events, plan.EventFailed, 1) {
		t.Errorf("events = %v, want failed:1", describe(events))
	}
}

func TestRun_KeepaliveStartsAfterTheHandoffAndStopsOnEveryExit(t *testing.T) {
	for _, exit := range []string{"success", "failure", "cancel"} {
		t.Run(exit, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var inHandoff atomic.Bool
			var probesDuringHandoff atomic.Int32
			r := &scriptedRunner{}
			r.handle = func(ctx context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
				switch {
				case isSudoProbe(s):
					if inHandoff.Load() {
						probesDuringHandoff.Add(1)
					}
				case labelOf(s) == handoffLabel:
					inHandoff.Store(true)
					time.Sleep(6 * keepaliveTick)
					inHandoff.Store(false)
				case labelOf(s) == "long":
					waitUntil(t, func() bool { return r.quietCalls() >= 3 })
					switch exit {
					case "failure":
						return failedStep("boom")
					case "cancel":
						cancel()
						<-ctx.Done()
						return plan.StepResult{Status: plan.StepFailed, Err: ctx.Err()}
					}
				}
				return plan.StepResult{Status: plan.StepOK}
			}
			inst := newRunInstaller(r)
			inst.KeepaliveEvery = keepaliveTick

			runInstallCtx(t, ctx, inst, []PlannedStep{sudoHandoff(), sudoProbe(), command("long")}, nil)

			if n := probesDuringHandoff.Load(); n != 0 {
				t.Errorf("%d keepalive probes ran before the handoff finished", n)
			}
			calls := r.quietCalls()
			time.Sleep(10 * keepaliveTick)
			if after := r.quietCalls(); after != calls {
				t.Errorf("keepalive still running after Run returned: %d -> %d probes", calls, after)
			}
		})
	}
}

const wantSudoExpired = "sudo credentials expired; run the install again"

// expiringKeepaliveRunner fails every keepalive probe and holds the "slow" step until the first failure is stored.
func expiringKeepaliveRunner(t *testing.T) *scriptedRunner {
	t.Helper()
	var probes atomic.Int32
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		switch {
		case isSudoProbe(s):
			if probes.Add(1) > 1 {
				return failedStep("sudo: a password is required")
			}
		case labelOf(s) == "slow":
			// The keepalive is sequential: probe 3 starting means probe 2 returned and the expiry flag is set.
			waitUntil(t, func() bool { return r.quietCalls() >= 3 })
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	return r
}

func TestRun_FailedKeepaliveFailsTheNextSudoStepWithoutRunningIt(t *testing.T) {
	cases := []struct {
		name string
		step plan.CommandStep
	}{
		{"sudo prefix", plan.CommandStep{Label: "after", Argv: []string{"systemctl"}, Sudo: true}},
		{"calls sudo itself", plan.CommandStep{Label: "after", Argv: []string{"makepkg", "-si"}, UsesSudo: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := expiringKeepaliveRunner(t)
			inst := newRunInstaller(r)
			inst.KeepaliveEvery = keepaliveTick

			res, events := runInstall(t, inst, []PlannedStep{sudoHandoff(), sudoProbe(), command("slow"), planned(PhaseServices, tc.step), recordStep()}, nil)

			if slices.Contains(r.commandLabels(), "after") {
				t.Error("sudo step ran with expired credentials")
			}
			if res.Err == nil || !strings.Contains(res.Err.Error(), wantSudoExpired) {
				t.Errorf("Err = %v, want message %q", res.Err, wantSudoExpired)
			}
			if !hasEvent(events, plan.EventDone, 2) || !hasEvent(events, plan.EventFailed, 3) {
				t.Errorf("events = %v, want the running step to finish and the next sudo step to fail", describe(events))
			}
		})
	}
}

func TestRun_RebootIsRefusedWhenTheSudoCredentialsExpired(t *testing.T) {
	r := expiringKeepaliveRunner(t)
	inst := newRunInstaller(r)
	inst.KeepaliveEvery = keepaliveTick
	ack := make(chan bool, 1)
	ack <- true

	res, _ := runInstall(t, inst, []PlannedStep{sudoHandoff(), sudoProbe(), command("slow"), recordStep(), rebootStep()}, ack)

	if slices.Contains(r.commandLabels(), "reboot") {
		t.Error("rebooted with expired credentials")
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), wantSudoExpired) {
		t.Errorf("Err = %v, want message %q", res.Err, wantSudoExpired)
	}
}

func TestRun_DryRunNeverRebootsAwaitsAckOrStartsTheKeepalive(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		if labelOf(s) == "slow" {
			time.Sleep(8 * keepaliveTick)
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	inst := newRunInstaller(r)
	inst.DryRun = true
	inst.KeepaliveEvery = keepaliveTick

	_, events := runInstall(t, inst, []PlannedStep{sudoHandoff(), sudoProbe(), command("slow"), recordStep(), rebootStep()}, nil)

	if slices.Contains(r.commandLabels(), "reboot") {
		t.Error("dry-run called the Runner for the reboot")
	}
	if slices.ContainsFunc(events, func(e plan.Event) bool { return e.Kind == plan.EventAwaitAck }) {
		t.Errorf("dry-run emitted await-ack: %v", describe(events))
	}
	if n := r.quietCalls(); n > 1 {
		t.Errorf("dry-run made %d quiet sudo calls, want only the planned probe", n)
	}
}

func TestRun_RebootWaitsForAcknowledgementAfterRecording(t *testing.T) {
	r := &scriptedRunner{}
	inst := newRunInstaller(r)
	events := make(chan plan.Event, eventBuffer)
	ack := make(chan bool)
	done := make(chan InstallResult, 1)
	go func() {
		done <- inst.Run(context.Background(), InstallPlan{Steps: []PlannedStep{command("one"), recordStep(), rebootStep()}}, events, ack)
	}()

	awaitEvent(t, events, plan.EventAwaitAck)
	time.Sleep(settle)

	if slices.Contains(r.commandLabels(), "reboot") {
		t.Fatal("rebooted before the acknowledgement")
	}
	select {
	case <-done:
		t.Fatal("Run returned before the acknowledgement")
	default:
	}
	logs := stepsOfType[plan.AppendLogStep](r.ran())
	if len(logs) != 1 || logs[0].Entry.Op != "install" || logs[0].Entry.Result != "ok" {
		t.Errorf("log record at await-ack = %+v, want one install entry with result ok", logs)
	}

	ack <- true
	select {
	case <-done:
	case <-time.After(runTimeout):
		t.Fatal("Run did not return after the acknowledgement")
	}
	if got := r.commandLabels(); !slices.Equal(got, []string{"one", "reboot"}) {
		t.Errorf("commands run = %v, want one then reboot", got)
	}
}

func TestRun_CancelWhileAwaitingAcknowledgementDoesNotReboot(t *testing.T) {
	r := &scriptedRunner{}
	inst := newRunInstaller(r)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan plan.Event, eventBuffer)
	done := make(chan InstallResult, 1)
	go func() {
		done <- inst.Run(ctx, InstallPlan{Steps: []PlannedStep{command("one"), recordStep(), rebootStep()}}, events, make(chan bool))
	}()

	awaitEvent(t, events, plan.EventAwaitAck)
	cancel()
	select {
	case <-done:
	case <-time.After(runTimeout):
		t.Fatal("Run did not return after cancel")
	}
	if slices.Contains(r.commandLabels(), "reboot") {
		t.Error("rebooted after cancel")
	}
}

const (
	gateID     = "spotify"
	gatePrompt = "Did Spotify open?"
)

func confirmGate() PlannedStep {
	return planned(PhaseTheme, plan.ConfirmStep{ID: gateID, Prompt: gatePrompt})
}

func gated(label string) PlannedStep {
	ps := planned(PhaseTheme, plan.CommandStep{Label: label, Argv: []string{label}})
	ps.After = gateID
	return ps
}

func answered(answer bool) <-chan bool {
	ack := make(chan bool, 1)
	ack <- answer
	return ack
}

func loggedItems(t *testing.T, r *scriptedRunner) []state.LogItem {
	t.Helper()
	logs := stepsOfType[plan.AppendLogStep](r.ran())
	if len(logs) != 1 {
		t.Fatalf("log records = %d, want 1", len(logs))
	}
	return logs[0].Entry.Items
}

func TestRun_ConfirmYesRunsTheGatedStepsAndLogsTheAnswer(t *testing.T) {
	r := &scriptedRunner{}
	steps := []PlannedStep{command("one"), confirmGate(), gated("apply"), recordStep()}

	res, events := runInstall(t, newRunInstaller(r), steps, answered(true))

	want := []string{"started:0", "done:0", "await-ack:1", "done:1", "started:2", "done:2", "started:3", "done:3"}
	if got := describe(events); !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
	if i := slices.IndexFunc(events, func(e plan.Event) bool { return e.Kind == plan.EventAwaitAck }); i < 0 || !events[i].YesNo || events[i].Line != gatePrompt {
		t.Errorf("await-ack event = %+v, want YesNo with the prompt as Line", events)
	}
	if got := r.commandLabels(); !slices.Equal(got, []string{"one", "apply"}) {
		t.Errorf("commands run = %v, want the confirm kept away from the Runner", got)
	}
	if got := loggedItems(t, r); !slices.Contains(got, state.LogItem{Path: gatePrompt, Kind: "confirm", Result: "yes"}) {
		t.Errorf("log items = %v, want a confirm item with result yes", got)
	}
	if res.Err != nil {
		t.Errorf("Err = %v", res.Err)
	}
}

func TestRun_ConfirmNoSkipsTheGatedStepsAndKeepsRunning(t *testing.T) {
	r := &scriptedRunner{}
	steps := []PlannedStep{confirmGate(), gated("apply"), command("after"), recordStep()}

	res, events := runInstall(t, newRunInstaller(r), steps, answered(false))

	if !hasEvent(events, plan.EventSkipped, 1) || hasEvent(events, plan.EventStarted, 1) {
		t.Errorf("events = %v, want the gated step skipped and never started", describe(events))
	}
	if got := r.commandLabels(); !slices.Equal(got, []string{"after"}) {
		t.Errorf("commands run = %v, want only the ungated step", got)
	}
	items := loggedItems(t, r)
	for _, want := range []state.LogItem{
		{Path: gatePrompt, Kind: "confirm", Result: "no"},
		{Path: "apply", Kind: "command", Result: "skipped"},
	} {
		if !slices.Contains(items, want) {
			t.Errorf("log items = %v, want %+v", items, want)
		}
	}
	if res.Err != nil {
		t.Errorf("Err = %v, want the run to continue after a no", res.Err)
	}
}

func TestRun_CancelAtTheConfirmPromptStopsWithoutRunningGatedSteps(t *testing.T) {
	r := &scriptedRunner{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan plan.Event, eventBuffer)
	done := make(chan InstallResult, 1)
	go func() {
		done <- newRunInstaller(r).Run(ctx, InstallPlan{Steps: []PlannedStep{confirmGate(), gated("apply"), recordStep()}}, events, make(chan bool, 1))
	}()

	awaitEvent(t, events, plan.EventAwaitAck)
	cancel()
	select {
	case res := <-done:
		if !errors.Is(res.Err, context.Canceled) {
			t.Errorf("Err = %v, want context.Canceled", res.Err)
		}
	case <-time.After(runTimeout):
		t.Fatal("Run did not return after cancel")
	}
	if slices.Contains(r.commandLabels(), "apply") {
		t.Error("gated step ran after cancel")
	}
}

func TestRun_DryRunSendsTheConfirmToTheRunnerAndMeetsTheGate(t *testing.T) {
	r := &scriptedRunner{}
	inst := newRunInstaller(r)
	inst.DryRun = true

	_, events := runInstall(t, inst, []PlannedStep{confirmGate(), gated("apply"), recordStep()}, nil)

	if got := stepsOfType[plan.ConfirmStep](r.ran()); len(got) != 1 {
		t.Errorf("Runner received %d confirm steps, want 1", len(got))
	}
	if !slices.Contains(r.commandLabels(), "apply") {
		t.Error("dry-run did not run the gated step through the Runner")
	}
	if slices.ContainsFunc(events, func(e plan.Event) bool { return e.Kind == plan.EventAwaitAck }) {
		t.Errorf("dry-run emitted await-ack: %v", describe(events))
	}
}

func TestRun_TwoInteractiveStepsStartOneKeepalive(t *testing.T) {
	var inFlight, peak atomic.Int32
	r := &scriptedRunner{}
	r.handle = func(ctx context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		switch {
		case isSudoProbe(s):
			peak.Store(max(peak.Load(), inFlight.Add(1)))
			<-ctx.Done()
			inFlight.Add(-1)
		case labelOf(s) == "slow":
			time.Sleep(8 * keepaliveTick)
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	inst := newRunInstaller(r)
	inst.KeepaliveEvery = keepaliveTick
	interactive := planned(PhaseTheme, plan.CommandStep{Label: "launch", Argv: []string{"launch"}, Interactive: true})

	runInstall(t, inst, []PlannedStep{sudoHandoff(), interactive, command("slow")}, nil)

	if got := peak.Load(); got != 1 {
		t.Errorf("concurrent keepalive probes peaked at %d, want 1 keepalive", got)
	}
}

func TestRun_ScrubsSecretValuesFromProgressLines(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
		if _, ok := s.(plan.WriteSecretsStep); ok {
			progress(plan.StepProgress{Line: "wrote API_KEY=" + secretSentinel + " ok"})
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	secrets := planned(PhaseEnv, plan.WriteSecretsStep{
		Path:    secretsPath,
		Entries: []plan.SecretEntry{{Name: "API_KEY", Value: secretSentinel}},
	})

	_, events := runInstall(t, newRunInstaller(r), []PlannedStep{secrets}, nil)

	if !slices.Contains(describe(events), "line:0:wrote API_KEY=*** ok") {
		t.Errorf("events = %v, want the value replaced by ***", describe(events))
	}
	for _, e := range events {
		if strings.Contains(e.Line, secretSentinel) {
			t.Errorf("secret value leaked in event %+v", e)
		}
	}
}

func TestRun_ScrubsALongerSecretThatStartsWithAShorterOne(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
		if _, ok := s.(plan.WriteSecretsStep); ok {
			progress(plan.StepProgress{Line: "wrote abc"})
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	secrets := planned(PhaseEnv, plan.WriteSecretsStep{
		Path:    secretsPath,
		Entries: []plan.SecretEntry{{Name: "A", Value: "ab"}, {Name: "B", Value: "abc"}},
	})

	_, events := runInstall(t, newRunInstaller(r), []PlannedStep{secrets}, nil)

	if !slices.Contains(describe(events), "line:0:wrote ***") {
		t.Errorf("events = %v, want the whole longer value replaced", describe(events))
	}
}

func TestRun_ScrubsSecretValuesFromTheFailureEventResultAndLog(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		if _, ok := s.(plan.WriteSecretsStep); ok {
			return failedStep("cannot write API_KEY=" + secretSentinel)
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	secrets := planned(PhaseEnv, plan.WriteSecretsStep{
		Path:    secretsPath,
		Entries: []plan.SecretEntry{{Name: "API_KEY", Value: secretSentinel}},
	})

	res, events := runInstall(t, newRunInstaller(r), []PlannedStep{secrets, recordStep()}, nil)

	if res.Err == nil || !strings.Contains(res.Err.Error(), "API_KEY=***") {
		t.Errorf("Err = %v, want the value replaced by ***", res.Err)
	}
	if res.Err != nil && strings.Contains(res.Err.Error(), secretSentinel) {
		t.Errorf("secret value leaked in the result: %v", res.Err)
	}
	for _, e := range events {
		if e.Err != nil && strings.Contains(e.Err.Error(), secretSentinel) {
			t.Errorf("secret value leaked in event %+v", e)
		}
	}
	for _, l := range stepsOfType[plan.AppendLogStep](r.ran()) {
		if strings.Contains(l.Entry.Error, secretSentinel) {
			t.Errorf("secret value leaked in the log entry: %q", l.Entry.Error)
		}
	}
}

func TestRun_BlockingPlanErrorsForbidRunningAnyStep(t *testing.T) {
	r := &scriptedRunner{}
	blocked := errors.New("blocked by an existing path: /home/u/.config/hypr")
	events := make(chan plan.Event, eventBuffer)

	res := newRunInstaller(r).Run(context.Background(), InstallPlan{Steps: []PlannedStep{command("one"), recordStep()}, Blocking: []error{blocked}}, events, nil)

	if !errors.Is(res.Err, blocked) {
		t.Errorf("Err = %v, want the blocking error", res.Err)
	}
	if len(r.ran()) != 0 {
		t.Errorf("runner received %v, want nothing", r.ran())
	}
	if _, open := <-events; open {
		t.Error("events channel is still open")
	}
}

func TestRun_LogsOneItemPerStepWithLabelKindAndResult(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		if labelOf(s) == "bad" {
			return failedStep("boom")
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	skipped := command("skipped")
	skipped.Skip = "already installed"
	steps := []PlannedStep{
		command("one"),
		skipped,
		planned(PhaseTheme, plan.WriteFileStep{Path: "/home/u/.config/nuydeck/theme/name"}),
		command("bad"),
		command("never"),
		recordStep(),
	}

	res, _ := runInstall(t, newRunInstaller(r), steps, nil)

	want := []state.LogItem{
		{Path: "one", Kind: "command", Result: "done"},
		{Path: "skipped", Kind: "command", Result: "skipped"},
		{Path: "/home/u/.config/nuydeck/theme/name", Kind: "file", Result: "done"},
		{Path: "bad", Kind: "command", Result: "failed"},
	}
	logs := stepsOfType[plan.AppendLogStep](r.ran())
	if len(logs) != 1 || !slices.Equal(logs[0].Entry.Items, want) {
		t.Errorf("log = %+v, want items %+v", logs, want)
	}
	if res.Err == nil || !strings.HasPrefix(res.Err.Error(), "bad: ") {
		t.Errorf("Err = %v, want the step label as prefix", res.Err)
	}
}

func TestRun_CancelRecordsAFailureUnderALiveContextAndStopsLaterSteps(t *testing.T) {
	cases := []struct {
		name  string
		steps []PlannedStep
	}{
		{"between steps", []PlannedStep{command("one"), command("two"), recordStep()}},
		{"after the last step", []PlannedStep{command("one"), recordStep()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var recordCtxErr error
			r := &scriptedRunner{}
			r.handle = func(ctx context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
				switch {
				case labelOf(s) == "one":
					cancel()
				case isLogStep(s):
					recordCtxErr = ctx.Err()
				}
				return plan.StepResult{Status: plan.StepOK}
			}

			res, _ := runInstallCtx(t, ctx, newRunInstaller(r), tc.steps, nil)

			if got := r.commandLabels(); !slices.Equal(got, []string{"one"}) {
				t.Errorf("commands run = %v, want one only", got)
			}
			logs := stepsOfType[plan.AppendLogStep](r.ran())
			if len(logs) != 1 || logs[0].Entry.Result != "failed" || !strings.Contains(logs[0].Entry.Error, "context canceled") {
				t.Errorf("log = %+v, want one failed entry naming the cancel", logs)
			}
			if recordCtxErr != nil {
				t.Errorf("Record ran under a cancelled context: %v", recordCtxErr)
			}
			if !errors.Is(res.Err, context.Canceled) {
				t.Errorf("Err = %v, want context canceled", res.Err)
			}
		})
	}
}

func TestRun_CancelDuringTheVerificationProbeReportsTheCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		if isSudoProbe(s) {
			cancel()
			return failedStep("signal: killed")
		}
		return plan.StepResult{Status: plan.StepOK}
	}

	res, _ := runInstallCtx(t, ctx, newRunInstaller(r), []PlannedStep{sudoHandoff(), sudoProbe(), command("pacman")}, nil)

	if !errors.Is(res.Err, context.Canceled) || errors.Is(res.Err, errSudoNotVisible) {
		t.Errorf("Err = %v, want the cancel and not the sudo visibility error", res.Err)
	}
}

func TestRun_FailedRecordBlocksTheReboot(t *testing.T) {
	r := &scriptedRunner{}
	r.handle = func(_ context.Context, s plan.Step, _ func(plan.StepProgress)) plan.StepResult {
		if isLogStep(s) {
			return failedStep("disk full")
		}
		return plan.StepResult{Status: plan.StepOK}
	}
	ack := make(chan bool, 1)
	ack <- true

	res, events := runInstall(t, newRunInstaller(r), []PlannedStep{command("one"), recordStep(), rebootStep()}, ack)

	if slices.Contains(r.commandLabels(), "reboot") || hasEvent(events, plan.EventAwaitAck, 2) {
		t.Errorf("reboot reached after a failed record: %v", describe(events))
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "disk full") {
		t.Errorf("Err = %v, want the record error", res.Err)
	}
}

func TestRun_DryRunReportsTheRebootAsSkipped(t *testing.T) {
	inst := newRunInstaller(&scriptedRunner{})
	inst.DryRun = true

	_, events := runInstall(t, inst, []PlannedStep{command("one"), recordStep(), rebootStep()}, nil)

	if !hasEvent(events, plan.EventSkipped, 2) {
		t.Errorf("events = %v, want skipped:2", describe(events))
	}
}

func isLogStep(s plan.Step) bool {
	_, ok := s.(plan.AppendLogStep)
	return ok
}
