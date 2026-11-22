package plan

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

type runnerFunc func(ctx context.Context, s Step, progress func(StepProgress)) StepResult

func (f runnerFunc) Run(ctx context.Context, s Step, progress func(StepProgress)) StepResult {
	return f(ctx, s, progress)
}

func cmdItem(label string) Item { return Item{Step: CommandStep{Label: label}} }

func logItem() Item { return Item{Step: AppendLogStep{}} }

func labelOf(s Step) string {
	label, _ := Describe(s)
	return label
}

// ran records the label of every step the runner receives; failing names the labels that fail.
func ran(failing ...string) (runnerFunc, *[]string) {
	var got []string
	r := func(_ context.Context, s Step, _ func(StepProgress)) StepResult {
		label := labelOf(s)
		got = append(got, label)
		if slices.Contains(failing, label) {
			return StepResult{Status: StepFailed, Err: errors.New(label + " broke")}
		}
		return StepResult{Status: StepOK}
	}
	return r, &got
}

func collect() (func(Event), *[]Event) {
	var events []Event
	return func(e Event) { events = append(events, e) }, &events
}

func kinds(events []Event) []EventKind {
	var out []EventKind
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return out
}

func TestApply_EmitsStartedLineDoneInOrderWithIndexAndTotal(t *testing.T) {
	runner := runnerFunc(func(_ context.Context, s Step, progress func(StepProgress)) StepResult {
		progress(StepProgress{Line: "working"})
		return StepResult{Status: StepOK}
	})
	emit, events := collect()

	res := Apply(t.Context(), runner, []Item{cmdItem("a"), cmdItem("b")}, Hooks{}, emit)

	if res.Err != nil {
		t.Fatalf("Err = %v", res.Err)
	}
	want := []EventKind{EventStarted, EventLine, EventDone, EventStarted, EventLine, EventDone}
	if got := kinds(*events); !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	first, last := (*events)[0], (*events)[5]
	if first.Index != 0 || first.Total != 2 || first.Label != "a" {
		t.Errorf("first event = %+v", first)
	}
	if last.Index != 1 || last.Total != 2 || last.Label != "b" {
		t.Errorf("last event = %+v", last)
	}
	if (*events)[1].Line != "working" {
		t.Errorf("line event = %+v", (*events)[1])
	}
}

func TestApply_SkipReasonReportsSkippedWithoutCallingTheRunner(t *testing.T) {
	runner, got := ran()
	emit, events := collect()
	items := []Item{{Step: CommandStep{Label: "a"}, Skip: "already installed"}, cmdItem("b")}

	res := Apply(t.Context(), runner, items, Hooks{}, emit)

	if res.Err != nil {
		t.Fatalf("Err = %v", res.Err)
	}
	if !slices.Equal(*got, []string{"b"}) {
		t.Errorf("runner saw %v, want [b]", *got)
	}
	if k := kinds(*events); k[0] != EventSkipped || (*events)[0].Index != 0 {
		t.Errorf("first event = %+v, want Skipped at index 0", (*events)[0])
	}
}

func TestApply_StopsAtFirstFailureAndNeverRunsLaterSteps(t *testing.T) {
	runner, got := ran("b")
	emit, events := collect()

	res := Apply(t.Context(), runner, []Item{cmdItem("a"), cmdItem("b"), cmdItem("c")}, Hooks{}, emit)

	if res.Err == nil || !strings.Contains(res.Err.Error(), "b broke") {
		t.Fatalf("Err = %v, want the failure of b", res.Err)
	}
	if !slices.Equal(*got, []string{"a", "b"}) {
		t.Errorf("runner saw %v, want [a b]", *got)
	}
	last := (*events)[len(*events)-1]
	if last.Kind != EventFailed || last.Index != 1 || last.Err == nil {
		t.Errorf("last event = %+v, want Failed at index 1 with Err", last)
	}
}

func TestApply_AlwaysStepRunsAfterAFailure(t *testing.T) {
	runner, got := ran("a")

	res := Apply(t.Context(), runner, []Item{cmdItem("a"), cmdItem("b"), logItem()}, Hooks{}, func(Event) {})

	if res.Err == nil {
		t.Fatal("Err = nil, want the failure of a")
	}
	if want := []string{"a", "record install log"}; !slices.Equal(*got, want) {
		t.Errorf("runner saw %v, want %v", *got, want)
	}
}

func TestApply_AlwaysStepRunsUnderALiveContextAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var ran []string
	var logCtxErr error
	runner := runnerFunc(func(ctx context.Context, s Step, _ func(StepProgress)) StepResult {
		ran = append(ran, labelOf(s))
		if _, ok := s.(AppendLogStep); ok {
			logCtxErr = ctx.Err()
			return StepResult{Status: StepOK}
		}
		cancel()
		return StepResult{Status: StepOK}
	})

	res := Apply(ctx, runner, []Item{cmdItem("a"), cmdItem("b"), logItem()}, Hooks{}, func(Event) {})

	if !errors.Is(res.Err, context.Canceled) {
		t.Errorf("Err = %v, want context.Canceled", res.Err)
	}
	if want := []string{"a", "record install log"}; !slices.Equal(ran, want) {
		t.Errorf("runner saw %v, want %v", ran, want)
	}
	if logCtxErr != nil {
		t.Errorf("always step ran under a cancelled context: %v", logCtxErr)
	}
}

func TestApply_ScrubsSecretsFromLinesFailureEventAndResult(t *testing.T) {
	secrets := Item{Step: secretsStep("hunter2")}
	runner := runnerFunc(func(_ context.Context, s Step, progress func(StepProgress)) StepResult {
		if _, ok := s.(WriteSecretsStep); ok {
			return StepResult{Status: StepOK}
		}
		progress(StepProgress{Line: "token=hunter2"})
		return StepResult{Status: StepFailed, Err: errors.New("denied hunter2")}
	})
	emit, events := collect()

	res := Apply(t.Context(), runner, []Item{secrets, cmdItem("a")}, Hooks{}, emit)

	var seen []string
	for _, e := range *events {
		seen = append(seen, e.Line)
		if e.Err != nil {
			seen = append(seen, e.Err.Error())
		}
	}
	seen = append(seen, res.Err.Error())
	for _, text := range seen {
		if strings.Contains(text, "hunter2") {
			t.Errorf("secret leaked in %q", text)
		}
	}
	if !slices.Contains(seen, "token=***") {
		t.Errorf("line was not masked as token=***: %v", seen)
	}
}

func TestApply_BeforeErrorFailsTheStepWithoutRunningItOrTheRest(t *testing.T) {
	runner, got := ran()
	emit, events := collect()
	veto := errors.New("sudo expired")
	hooks := Hooks{Before: func(_ context.Context, index int, _ Step) error {
		if index == 1 {
			return veto
		}
		return nil
	}}

	res := Apply(t.Context(), runner, []Item{cmdItem("a"), cmdItem("b"), cmdItem("c")}, hooks, emit)

	if !errors.Is(res.Err, veto) {
		t.Errorf("Err = %v, want the veto", res.Err)
	}
	if !slices.Equal(*got, []string{"a"}) {
		t.Errorf("runner saw %v, want [a]", *got)
	}
	last := (*events)[len(*events)-1]
	if last.Kind != EventFailed || last.Index != 1 {
		t.Errorf("last event = %+v, want Failed at index 1", last)
	}
}

func TestApply_AfterRunsAfterEachExecutedStepWithItsResult(t *testing.T) {
	runner, _ := ran()
	var calls []string
	hooks := Hooks{After: func(_ context.Context, index int, s Step, r StepResult) {
		if r.Status != StepOK {
			t.Errorf("After got status %v", r.Status)
		}
		calls = append(calls, labelOf(s))
	}}
	items := []Item{cmdItem("a"), {Step: CommandStep{Label: "b"}, Skip: "no"}, cmdItem("c")}

	Apply(t.Context(), runner, items, hooks, func(Event) {})

	if want := []string{"a", "c"}; !slices.Equal(calls, want) {
		t.Errorf("After saw %v, want %v", calls, want)
	}
}
