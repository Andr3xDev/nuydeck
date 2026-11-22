package plan

import (
	"cmp"
	"context"
	"errors"
	"strings"
)

// Item is one step of an Apply; a non-empty Skip reports it as skipped without running it.
type Item struct {
	Step Step
	Skip string
}

// Hooks let the caller veto a step before it runs and observe it after it succeeds.
type Hooks struct {
	Before func(ctx context.Context, index int, s Step) error
	After  func(ctx context.Context, index int, s Step, r StepResult)
}

// Always marks a step that still runs after a failure or a cancel.
type Always interface {
	always()
}

func (AppendLogStep) always() {}

// Result is the outcome of Apply; Err is the first failure or the context error.
type Result struct {
	Err error
}

// Apply runs the items in order and stops at the first failure; Always steps run last, under a live context.
func Apply(ctx context.Context, runner Runner, items []Item, hooks Hooks, emit func(Event)) Result {
	steps := make([]Step, len(items))
	for i, it := range items {
		steps[i] = it.Step
	}
	scrub := SecretScrubber(steps)

	runItem := func(runCtx context.Context, index int, it Item) error {
		label, _ := Describe(it.Step)
		event := func(kind EventKind) Event {
			return Event{Index: index, Total: len(items), Label: label, Kind: kind}
		}
		fail := func(err error) error {
			err = scrubError(scrub, err)
			e := event(EventFailed)
			e.Err = err
			emit(e)
			return err
		}
		if it.Skip != "" {
			emit(event(EventSkipped))
			return nil
		}
		if hooks.Before != nil {
			if err := hooks.Before(runCtx, index, it.Step); err != nil {
				return fail(err)
			}
		}
		emit(event(EventStarted))
		res := runner.Run(runCtx, it.Step, func(sp StepProgress) {
			if sp.Line != "" {
				e := event(EventLine)
				e.Line = Redact(scrub, sp.Line)
				emit(e)
			}
		})
		if res.Status == StepFailed {
			return fail(cmp.Or(res.Err, errors.New(cmp.Or(res.Detail, "step failed"))))
		}
		if hooks.After != nil {
			hooks.After(runCtx, index, it.Step, res)
		}
		emit(event(EventDone))
		return nil
	}

	var runErr error
	for index, it := range items {
		if _, always := it.Step.(Always); always {
			continue
		}
		if runErr = ctx.Err(); runErr != nil {
			break
		}
		if runErr = runItem(ctx, index, it); runErr != nil {
			break
		}
	}
	if runErr == nil {
		runErr = ctx.Err()
	}
	for index, it := range items {
		if _, always := it.Step.(Always); !always {
			continue
		}
		if err := runItem(context.WithoutCancel(ctx), index, it); err != nil && runErr == nil {
			runErr = err
		}
	}
	if runErr != nil {
		runErr = scrubError(scrub, runErr)
	}
	return Result{Err: runErr}
}

// scrubError keeps the original error, and so errors.Is, unless it holds a secret value.
func scrubError(scrub *strings.Replacer, err error) error {
	if msg := Redact(scrub, err.Error()); msg != err.Error() {
		return errors.New(msg)
	}
	return err
}
