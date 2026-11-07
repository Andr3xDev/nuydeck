package app

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

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// defaultKeepaliveEvery stays well under sudo's default 5 minute timestamp_timeout.
const defaultKeepaliveEvery = 60 * time.Second

const secretMask = "***"

var (
	errSudoNotVisible = errors.New("sudo credentials are not visible to child processes (sudo timestamp_type); see the install log")
	errSudoExpired    = errors.New("sudo credentials expired; run the install again")
)

// Run executes the plan in order and closes events when it returns.
// It stops at the first failure; the Record step still runs, the Reboot step never does after a failure, cancel or in dry-run.
func (i *Installer) Run(parent context.Context, p InstallPlan, events chan<- InstallEvent, ack <-chan struct{}) InstallResult {
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
	var items []domain.LogItem
	scrub := secretScrubber(p.Steps)
	emit := func(e InstallEvent) {
		select {
		case events <- e:
		case <-ctx.Done():
		}
	}
	progress := func(index int) func(domain.StepProgress) {
		return func(sp domain.StepProgress) {
			if sp.Line != "" {
				emit(InstallEvent{Index: index, Kind: EventLine, Line: scrub.Replace(sp.Line)})
			}
		}
	}
	runStep := func(runCtx context.Context, index int, ps domain.PlannedStep) error {
		label, kind := describeStep(ps.Step)
		logItem := func(result string) {
			if ps.Phase < domain.PhaseRecord {
				items = append(items, domain.LogItem{Path: label, Kind: kind, Result: result})
			}
		}
		fail := func(err error) error {
			logItem("failed")
			err = scrubError(scrub, fmt.Errorf("%s: %w", label, err))
			emit(InstallEvent{Index: index, Kind: EventFailed, Err: err})
			return err
		}
		if ps.Skip != "" {
			logItem("skipped")
			emit(InstallEvent{Index: index, Kind: EventSkipped})
			return nil
		}
		if needsSudo(ps.Step) && sudoExpired.Load() {
			return fail(errSudoExpired)
		}
		emit(InstallEvent{Index: index, Kind: EventStarted})
		res := i.Runner.Run(runCtx, ps.Step, progress(index))
		if res.Status != domain.StepFailed {
			logItem("done")
			emit(InstallEvent{Index: index, Kind: EventDone})
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
		if ps.Phase >= domain.PhaseRecord {
			break
		}
		if runErr = ctx.Err(); runErr != nil {
			break
		}
		if runErr = runStep(ctx, index, ps); runErr != nil {
			break
		}
		if c, ok := ps.Step.(domain.CommandStep); ok && c.Interactive && !i.DryRun {
			i.startKeepalive(ctx, &keepalive, &sudoExpired)
		}
	}
	if runErr == nil {
		runErr = ctx.Err()
	}

	// The record is persisted even when the run was cancelled.
	for index, ps := range p.Steps {
		if entry, ok := ps.Step.(domain.AppendLogStep); ok && ps.Phase == domain.PhaseRecord {
			ps.Step = domain.AppendLogStep{Entry: i.finishedEntry(entry.Entry, items, runErr)}
			if err := runStep(context.WithoutCancel(ctx), index, ps); err != nil && runErr == nil {
				runErr = err
			}
		}
	}
	if runErr != nil {
		return InstallResult{Err: runErr}
	}

	for index, ps := range p.Steps {
		if ps.Phase != domain.PhaseReboot {
			continue
		}
		if i.DryRun {
			emit(InstallEvent{Index: index, Kind: EventSkipped})
			break
		}
		emit(InstallEvent{Index: index, Kind: EventAwaitAck})
		select {
		case <-ack:
		case <-ctx.Done():
			return InstallResult{Err: ctx.Err()}
		}
		return InstallResult{Err: runStep(ctx, index, ps)}
	}
	return InstallResult{}
}

// startKeepalive refreshes the sudo timestamp until ctx ends. A failed refresh does not abort the running step;
// it only fails the next step that needs sudo.
func (i *Installer) startKeepalive(ctx context.Context, wg *sync.WaitGroup, expired *atomic.Bool) {
	ticker := time.NewTicker(cmp.Or(i.KeepaliveEvery, defaultKeepaliveEvery))
	probe := sudoProbeStep("keep sudo alive")
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if i.Runner.Run(ctx, probe, nil).Status == domain.StepFailed {
					expired.Store(true)
				}
			}
		}
	}()
}

func (i *Installer) finishedEntry(entry domain.LogEntry, items []domain.LogItem, runErr error) domain.LogEntry {
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
func describeStep(s domain.Step) (label, kind string) {
	switch s := s.(type) {
	case domain.CommandStep:
		return s.Label, "command"
	case domain.WriteFileStep:
		return s.Path, "file"
	case domain.SymlinkStep:
		return s.Path, "link"
	case domain.WriteSecretsStep:
		return s.Path, "secrets"
	case domain.CopyStep:
		return string(s.Entry.Root) + ":" + s.Entry.Path, "copy"
	case domain.BackupStep:
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

func needsSudo(s domain.Step) bool {
	switch s := s.(type) {
	case domain.CommandStep:
		return s.Sudo || s.UsesSudo
	case domain.WriteFileStep:
		return s.Privileged
	}
	return false
}

// isVerificationProbe is the quiet `sudo -n -v` that follows the handoff in the privilege phase.
func isVerificationProbe(ps domain.PlannedStep) bool {
	c, ok := ps.Step.(domain.CommandStep)
	return ok && ps.Phase == domain.PhasePrivilege && c.Quiet && slices.Equal(c.Argv, sudoProbeStep("").Argv)
}

// secretScrubber masks every secret value of the plan; values never reach argv or output by design, this is the second line.
func secretScrubber(steps []domain.PlannedStep) *strings.Replacer {
	var pairs []string
	for _, ps := range steps {
		if w, ok := ps.Step.(domain.WriteSecretsStep); ok {
			for _, e := range w.Entries {
				if e.Value != "" {
					pairs = append(pairs, e.Value.Reveal(), secretMask)
				}
			}
		}
	}
	return strings.NewReplacer(pairs...)
}
