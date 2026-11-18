package install

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

var (
	errSudoNotVisible = errors.New("sudo credentials are not visible to child processes (sudo timestamp_type); see the install log")
	errSudoExpired    = errors.New("sudo credentials expired; run the install again")
)

// Run executes the plan in order and closes events when it returns.
// It stops at the first failure; the Record step still runs, the Reboot step never does after a failure, cancel or in dry-run.
func (i *Installer) Run(parent context.Context, p InstallPlan, events chan<- plan.Event, ack <-chan bool) InstallResult {
	defer close(events)
	if len(p.Blocking) > 0 {
		return InstallResult{Err: errors.Join(p.Blocking...)}
	}
	ctx, cancel := context.WithCancel(parent)
	var keepalive sync.WaitGroup
	defer func() {
		cancel()
		keepalive.Wait()
	}()

	var sudoExpired atomic.Bool
	var items []state.LogItem
	scrub := plan.SecretScrubber(slices.Collect(func(yield func(plan.Step) bool) {
		for _, ps := range p.Steps {
			if !yield(ps.Step) {
				return
			}
		}
	}))
	emit := func(e plan.Event) {
		select {
		case events <- e:
		case <-ctx.Done():
		}
	}
	progress := func(index int) func(plan.StepProgress) {
		return func(sp plan.StepProgress) {
			if sp.Line != "" {
				emit(plan.Event{Index: index, Kind: plan.EventLine, Line: plan.Redact(scrub, sp.Line)})
			}
		}
	}
	softFailed := map[string]bool{}
	runStep := func(runCtx context.Context, index int, ps PlannedStep) error {
		label, kind := plan.Describe(ps.Step)
		logItem := func(result string) {
			if ps.Phase < PhaseRecord {
				items = append(items, state.LogItem{Path: label, Kind: kind, Result: result})
			}
		}
		fail := func(err error) error {
			logItem("failed")
			err = scrubError(scrub, fmt.Errorf("%s: %w", label, err))
			if ps.Optional {
				softFailed[ps.ID] = true
				emit(plan.Event{Index: index, Kind: plan.EventLine, Line: "warning: " + err.Error()})
				emit(plan.Event{Index: index, Kind: plan.EventSkipped})
				return nil
			}
			emit(plan.Event{Index: index, Kind: plan.EventFailed, Err: err})
			return err
		}
		if ps.Skip != "" {
			logItem("skipped")
			emit(plan.Event{Index: index, Kind: plan.EventSkipped})
			return nil
		}
		if needsSudo(ps.Step) && sudoExpired.Load() {
			return fail(errSudoExpired)
		}
		emit(plan.Event{Index: index, Kind: plan.EventStarted})
		res := i.Runner.Run(runCtx, ps.Step, progress(index))
		if res.Status != plan.StepFailed {
			logItem("done")
			emit(plan.Event{Index: index, Kind: plan.EventDone})
			return nil
		}
		err := cmp.Or(res.Err, errors.New(cmp.Or(res.Detail, "step failed")))
		if isVerificationProbe(ps) {
			err = cmp.Or(runCtx.Err(), errSudoNotVisible)
		}
		return fail(err)
	}

	// confirm asks the owner a yes/no question; the run stops only if ctx ends first.
	declined := map[string]bool{}
	declineNotice := func(index int, notice string) {
		if notice != "" {
			emit(plan.Event{Index: index, Kind: plan.EventLine, Line: notice})
		}
	}
	confirm := func(index int, ps PlannedStep, c plan.ConfirmStep) error {
		emit(plan.Event{Index: index, Kind: plan.EventAwaitAck, Line: c.Prompt, YesNo: true})
		var yes bool
		select {
		case yes = <-ack:
		case <-ctx.Done():
			return ctx.Err()
		}
		declined[c.ID] = !yes
		label, kind := plan.Describe(ps.Step)
		result := "no"
		if yes {
			result = "yes"
		}
		items = append(items, state.LogItem{Path: label, Kind: kind, Result: result})
		emit(plan.Event{Index: index, Kind: plan.EventDone})
		if !yes {
			declineNotice(index, c.DeclineNotice)
		}
		return nil
	}

	var runErr error
	keepaliveStarted := false
	for index, ps := range p.Steps {
		if ps.Phase >= PhaseRecord {
			break
		}
		if runErr = ctx.Err(); runErr != nil {
			break
		}
		if c, ok := ps.Step.(plan.ConfirmStep); ok {
			if ps.After != "" && softFailed[ps.After] {
				ps.Skip = "launch failed"
				declined[c.ID] = true
				declineNotice(index, strings.TrimSpace("launch failed. "+c.DeclineNotice))
			} else if ps.Skip == "" && !i.DryRun {
				if runErr = confirm(index, ps, c); runErr != nil {
					break
				}
				continue
			}
		}
		if ps.After != "" && declined[ps.After] {
			ps.Skip = "not confirmed"
		}
		if runErr = runStep(ctx, index, ps); runErr != nil {
			break
		}
		if c, ok := ps.Step.(plan.CommandStep); ok && c.Interactive && !i.DryRun && !keepaliveStarted {
			keepaliveStarted = true
			i.startKeepalive(ctx, &keepalive, &sudoExpired)
		}
	}
	if runErr == nil {
		runErr = ctx.Err()
	}

	// The record is persisted even when the run was cancelled.
	for index, ps := range p.Steps {
		if entry, ok := ps.Step.(plan.AppendLogStep); ok && ps.Phase == PhaseRecord {
			ps.Step = plan.AppendLogStep{Entry: i.finishedEntry(entry.Entry, items, runErr)}
			if err := runStep(context.WithoutCancel(ctx), index, ps); err != nil && runErr == nil {
				runErr = err
			}
		}
	}
	if runErr != nil {
		return InstallResult{Err: runErr}
	}

	for index, ps := range p.Steps {
		if ps.Phase != PhaseReboot {
			continue
		}
		if i.DryRun {
			emit(plan.Event{Index: index, Kind: plan.EventSkipped})
			break
		}
		emit(plan.Event{Index: index, Kind: plan.EventAwaitAck})
		select {
		case <-ack:
		case <-ctx.Done():
			return InstallResult{Err: ctx.Err()}
		}
		return InstallResult{Err: runStep(ctx, index, ps)}
	}
	return InstallResult{}
}

func (i *Installer) finishedEntry(entry state.LogEntry, items []state.LogItem, runErr error) state.LogEntry {
	entry.Time = i.Now().Format(time.RFC3339)
	entry.DryRun = i.DryRun
	entry.Items = items
	entry.Result = "ok"
	if runErr != nil {
		entry.Result, entry.Error = "failed", runErr.Error()
	}
	return entry
}

// scrubError keeps the original error, and so errors.Is, unless it holds a secret value.
func scrubError(scrub *strings.Replacer, err error) error {
	if msg := plan.Redact(scrub, err.Error()); msg != err.Error() {
		return errors.New(msg)
	}
	return err
}

func needsSudo(s plan.Step) bool {
	switch s := s.(type) {
	case plan.CommandStep:
		return s.Sudo || s.UsesSudo
	case plan.WriteFileStep:
		return s.Privileged
	}
	return false
}

// isVerificationProbe is the quiet `sudo -n -v` that follows the handoff in the privilege phase.
func isVerificationProbe(ps PlannedStep) bool {
	c, ok := ps.Step.(plan.CommandStep)
	return ok && ps.Phase == PhasePrivilege && c.Quiet && slices.Equal(c.Argv, plan.SudoProbeStep("").Argv)
}
