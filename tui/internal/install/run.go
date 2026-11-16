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

const secretMask = "***"

var (
	errSudoNotVisible = errors.New("sudo credentials are not visible to child processes (sudo timestamp_type); see the install log")
	errSudoExpired    = errors.New("sudo credentials expired; run the install again")
)

// Run executes the plan in order and closes events when it returns.
// It stops at the first failure; the Record step still runs, the Reboot step never does after a failure, cancel or in dry-run.
func (i *Installer) Run(parent context.Context, p InstallPlan, events chan<- plan.InstallEvent, ack <-chan struct{}) InstallResult {
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
	scrub := secretScrubber(p.Steps)
	emit := func(e plan.InstallEvent) {
		select {
		case events <- e:
		case <-ctx.Done():
		}
	}
	progress := func(index int) func(plan.StepProgress) {
		return func(sp plan.StepProgress) {
			if sp.Line != "" {
				emit(plan.InstallEvent{Index: index, Kind: plan.EventLine, Line: scrub.Replace(sp.Line)})
			}
		}
	}
	runStep := func(runCtx context.Context, index int, ps PlannedStep) error {
		label, kind := describeStep(ps.Step)
		logItem := func(result string) {
			if ps.Phase < PhaseRecord {
				items = append(items, state.LogItem{Path: label, Kind: kind, Result: result})
			}
		}
		fail := func(err error) error {
			logItem("failed")
			err = scrubError(scrub, fmt.Errorf("%s: %w", label, err))
			emit(plan.InstallEvent{Index: index, Kind: plan.EventFailed, Err: err})
			return err
		}
		if ps.Skip != "" {
			logItem("skipped")
			emit(plan.InstallEvent{Index: index, Kind: plan.EventSkipped})
			return nil
		}
		if needsSudo(ps.Step) && sudoExpired.Load() {
			return fail(errSudoExpired)
		}
		emit(plan.InstallEvent{Index: index, Kind: plan.EventStarted})
		res := i.Runner.Run(runCtx, ps.Step, progress(index))
		if res.Status != plan.StepFailed {
			logItem("done")
			emit(plan.InstallEvent{Index: index, Kind: plan.EventDone})
			return nil
		}
		err := cmp.Or(res.Err, errors.New(cmp.Or(res.Detail, "step failed")))
		if isVerificationProbe(ps) {
			err = cmp.Or(runCtx.Err(), errSudoNotVisible)
		}
		return fail(err)
	}

	var runErr error
	for index, ps := range p.Steps {
		if ps.Phase >= PhaseRecord {
			break
		}
		if runErr = ctx.Err(); runErr != nil {
			break
		}
		if runErr = runStep(ctx, index, ps); runErr != nil {
			break
		}
		if c, ok := ps.Step.(plan.CommandStep); ok && c.Interactive && !i.DryRun {
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
			emit(plan.InstallEvent{Index: index, Kind: plan.EventSkipped})
			break
		}
		emit(plan.InstallEvent{Index: index, Kind: plan.EventAwaitAck})
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

// describeStep names a step for the log without argv or secret values.
func describeStep(s plan.Step) (label, kind string) {
	switch s := s.(type) {
	case plan.CommandStep:
		return s.Label, "command"
	case plan.WriteFileStep:
		return s.Path, "file"
	case plan.SymlinkStep:
		return s.Path, "link"
	case plan.WriteSecretsStep:
		return s.Path, "secrets"
	case plan.CopyStep:
		return string(s.Entry.Root) + ":" + s.Entry.Path, "copy"
	case plan.BackupStep:
		return "back up overwritten files", "backup"
	}
	return fmt.Sprintf("%T", s), "step"
}

// scrubError keeps the original error, and so errors.Is, unless it holds a secret value.
func scrubError(scrub *strings.Replacer, err error) error {
	if msg := scrub.Replace(err.Error()); msg != err.Error() {
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

// secretScrubber masks every secret value of the plan; values never reach argv or output by design, this is the second line.
func secretScrubber(steps []PlannedStep) *strings.Replacer {
	var values []string
	for _, ps := range steps {
		if w, ok := ps.Step.(plan.WriteSecretsStep); ok {
			for _, e := range w.Entries {
				if e.Value != "" {
					values = append(values, e.Value.Reveal())
				}
			}
		}
	}
	// NewReplacer matches in argument order, so a longer value must come before its prefix.
	slices.SortStableFunc(values, func(a, b string) int { return len(b) - len(a) })
	pairs := make([]string, 0, 2*len(values))
	for _, v := range values {
		pairs = append(pairs, v, secretMask)
	}
	return strings.NewReplacer(pairs...)
}
