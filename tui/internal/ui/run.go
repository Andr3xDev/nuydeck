package ui

import (
	"context"
	"slices"
	"sync/atomic"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	maxOutputLines  = 2000
	runNoticeText   = "a run is in progress, ctrl+c cancels"
	runAckText      = "Press enter to continue, ctrl+c to quit."
	runAnswerText   = "y yes, n or esc no, ctrl+c cancels the run."
	runCloseText    = "Press esc to close, q to quit."
	runTitlePrefix  = "Output - "
	runFinishedText = "Finished."
)

// runEnd is how a run's work ended; err is nil on success. msg, when set, is broadcast to every screen once the run ends.
type runEnd struct {
	err error
	msg tea.Msg
}

// runRequest asks the shell to run work in the output pane. ack carries the reboot confirmation, if the work waits for one.
// steps adds a status mark per step above the output; footer replaces the default hint under it when it returns lines.
type runRequest struct {
	title  string
	work   func(ctx context.Context, emit func(plan.Event)) runEnd
	ack    chan bool
	steps  []string
	footer func(th theme, cw int, r *run) []string
}

type stepStatus int

const (
	statusPending stepStatus = iota
	statusRunning
	statusDone
	statusSkipped
	statusFailed
)

const (
	markWidth       = 3 // "[x]"
	stepListShare   = 3 // the step list takes at most 1/3 of the pane
	minStepListRows = 3
)

// runClosed tells every screen the output pane was dismissed.
type runClosed struct{ err error }

// returnToDashboardMsg sends the shell back to the dashboard.
type returnToDashboardMsg struct{}

type (
	runEventMsg struct{ event plan.Event }
	runDoneMsg  struct{ end runEnd }
)

type outLine struct{ text string }

// run is the output pane state; its text stays raw and is wrapped (and so sanitized) when drawn.
type run struct {
	title    string
	cancel   context.CancelFunc
	events   chan tea.Msg
	ack      chan bool
	lines    []outLine // capped at maxOutputLines, oldest dropped first
	done     bool
	awaitAck bool
	question string // the y/n question being asked, empty at a plain acknowledgement
	notice   bool
	follow   bool
	scroll   scroller
	steps    []string
	statuses []stepStatus
	current  int
	footer   func(th theme, cw int, r *run) []string
	err      error // how the work ended, set when done
	// started and finished let WaitInstall tell a run whose command never executed from one still winding down.
	started  atomic.Bool
	finished chan struct{}
}

// newRun creates the handle synchronously so a quit before the command executes sees an unstarted run.
func newRun(req runRequest, cancel context.CancelFunc) *run {
	return &run{
		title: req.title, cancel: cancel, events: make(chan tea.Msg), ack: req.ack, follow: true, finished: make(chan struct{}),
		steps: req.steps, statuses: make([]stepStatus, len(req.steps)), footer: req.footer,
	}
}

// start returns the command that launches work; ctx is the Root context, so a quit unblocks pending sends.
func (r *run) start(ctx, workCtx context.Context, work func(context.Context, func(plan.Event)) runEnd) tea.Cmd {
	return func() tea.Msg {
		r.started.Store(true)
		go func() {
			defer close(r.finished)
			send := func(m tea.Msg) {
				select {
				case r.events <- m:
				case <-ctx.Done():
				}
			}
			end := work(workCtx, func(e plan.Event) { send(runEventMsg{e}) })
			send(runDoneMsg{end})
		}()
		return <-r.events
	}
}

// next reads the following message of the run; the model re-arms it until runDoneMsg.
func (r *run) next() tea.Cmd { return func() tea.Msg { return <-r.events } }

func (r *run) push(lines ...outLine) {
	r.lines = append(r.lines, lines...)
	if drop := len(r.lines) - maxOutputLines; drop > 0 {
		r.lines = slices.Delete(r.lines, 0, drop)
	}
}

// mark records a step event in the status list; line events and unknown steps are ignored.
func (r *run) mark(e plan.Event) {
	if e.Kind == plan.EventLine || e.Index < 0 || e.Index >= len(r.statuses) {
		return
	}
	r.current = e.Index
	switch e.Kind {
	case plan.EventStarted, plan.EventAwaitAck:
		r.statuses[e.Index] = statusRunning
	case plan.EventDone:
		r.statuses[e.Index] = statusDone
	case plan.EventSkipped:
		r.statuses[e.Index] = statusSkipped
	case plan.EventFailed:
		r.statuses[e.Index] = statusFailed
	}
}

func (r *run) add(e plan.Event) {
	r.mark(e)
	switch {
	case e.Kind == plan.EventAwaitAck:
		r.awaitAck, r.notice, r.question = true, false, ""
		if e.YesNo {
			r.question = e.Line
			return
		}
	case e.Kind == plan.EventLine:
		r.push(outLine{e.Line})
	case e.Kind == plan.EventFailed && e.Err != nil:
		r.push(outLine{"failed: " + e.Err.Error()})
	}
}

func (r *run) finish(end runEnd) {
	r.done, r.awaitAck, r.notice, r.err = true, false, false, end.err
	if end.err != nil {
		r.push(outLine{"error: " + end.err.Error()})
		return
	}
	r.push(outLine{runFinishedText})
}

// sendAnswer leaves the prompt; the channel holds one answer, so a second press cannot block.
func (r *run) sendAnswer(yes bool) {
	r.awaitAck = false
	select {
	case r.ack <- yes:
	default:
	}
}

func (r *run) scrollBy(delta int) {
	if delta < 0 {
		r.follow = false
	}
	r.scroll.by(delta)
}

// hint is the one line under the output: what the keys do now.
func (r *run) hint() string {
	switch {
	case r.notice:
		return runNoticeText
	case r.awaitAck && r.question != "":
		return r.question + " " + runAnswerText
	case r.awaitAck:
		return runAckText
	case r.done:
		return runCloseText
	}
	return ""
}

// keys are the bindings the pane answers to now, for the keybar.
func (r *run) keys(k keyMap) []key.Binding {
	scroll := k.move()
	switch {
	case r.done:
		return append(scroll, described(k.Back, "close"))
	case r.awaitAck && r.question != "":
		return append(scroll, k.Yes, k.No, k.CancelRun)
	case r.awaitAck:
		return append(scroll, described(k.Confirm, "continue"), described(k.CancelRun, "quit"))
	}
	return append(scroll, k.CancelRun)
}

func statusMark(th theme, st stepStatus) string {
	switch st {
	case statusRunning:
		return "[>]"
	case statusDone:
		return "[x]"
	case statusSkipped:
		return "[-]"
	case statusFailed:
		return th.err.Render("[!]")
	}
	return "[ ]"
}

// stepList is the status list, windowed around the current step, followed by a blank line.
func (r *run) stepList(th theme, cw, inner int) []string {
	if len(r.steps) == 0 {
		return nil
	}
	rows := make([]string, len(r.steps))
	for i, label := range r.steps {
		rows[i] = statusMark(th, r.statuses[i]) + " " + truncate(sanitizeLine(label), max(cw-markWidth-1, 0))
	}
	return append(windowAround(rows, r.current, min(len(rows), max(inner/stepListShare, minStepListRows))), "")
}

// footerLines is the hook's text, or the default hint while the owner has a notice to read or the hook has nothing to say.
func (r *run) footerLines(th theme, cw int) []string {
	if r.footer != nil && !r.notice {
		if lines := r.footer(th, cw, r); lines != nil {
			return lines
		}
	}
	return styled(th.muted, wrap(r.hint(), cw))
}

// view re-wraps every line each frame; shortcut: fine at 2000 lines, cache wrapped lines if it shows in profiling.
func (r *run) view(th theme, w, h int) string {
	cw := contentWidth(w)
	var lines []string
	for _, l := range r.lines {
		lines = append(lines, styled(th.plain, wrap(l.text, cw))...)
	}
	if r.follow {
		r.scroll.offset = len(lines)
	}
	inner := innerHeight(h)
	head, foot := r.stepList(th, cw, inner), r.footerLines(th, cw)
	room := max(inner-len(head)-len(foot), 1)
	total := len(lines)
	lines = r.scroll.window(lines, room)
	r.follow = r.follow || r.scroll.offset+room >= total
	for len(lines) < room {
		lines = append(lines, "")
	}
	return screenBox(th, runTitlePrefix+sanitizeLine(r.title), slices.Concat(head, lines, foot), w, h)
}
