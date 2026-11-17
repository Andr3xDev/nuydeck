package ui

import (
	"context"
	"slices"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	maxOutputLines  = 2000
	runNoticeText   = "a run is in progress, ctrl+c cancels"
	runAckText      = "Press enter to continue, ctrl+c to quit."
	runCloseText    = "Press esc to close, q to quit."
	runTitlePrefix  = "Output - "
	runFinishedText = "Finished."
)

// runEnd is how a run's work ended; err is nil on success.
type runEnd struct{ err error }

// runRequest asks the shell to run work in the output pane. ack carries the reboot confirmation, if the work waits for one.
type runRequest struct {
	title string
	work  func(ctx context.Context, emit func(plan.InstallEvent)) runEnd
	ack   chan struct{}
}

// runClosed tells every screen the output pane was dismissed.
type runClosed struct{}

type (
	runEventMsg struct{ event plan.InstallEvent }
	runDoneMsg  struct{ end runEnd }
)

type outLine struct{ text string }

// run is the output pane state; its text stays raw and is wrapped (and so sanitized) when drawn.
type run struct {
	title    string
	cancel   context.CancelFunc
	events   chan tea.Msg
	ack      chan struct{}
	lines    []outLine // capped at maxOutputLines, oldest dropped first
	done     bool
	awaitAck bool
	notice   bool
	follow   bool
	scroll   scroller
	// started and finished let WaitInstall tell a run whose command never executed from one still winding down.
	started  atomic.Bool
	finished chan struct{}
}

// newRun creates the handle synchronously so a quit before the command executes sees an unstarted run.
func newRun(req runRequest, cancel context.CancelFunc) *run {
	return &run{title: req.title, cancel: cancel, events: make(chan tea.Msg), ack: req.ack, follow: true, finished: make(chan struct{})}
}

// start returns the command that launches work; ctx is the Root context, so a quit unblocks pending sends.
func (r *run) start(ctx, workCtx context.Context, work func(context.Context, func(plan.InstallEvent)) runEnd) tea.Cmd {
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
			end := work(workCtx, func(e plan.InstallEvent) { send(runEventMsg{e}) })
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

func (r *run) add(e plan.InstallEvent) {
	switch {
	case e.Kind == plan.EventAwaitAck:
		r.awaitAck = true
	case e.Kind == plan.EventLine:
		r.push(outLine{e.Line})
	case e.Kind == plan.EventFailed && e.Err != nil:
		r.push(outLine{"failed: " + e.Err.Error()})
	}
}

func (r *run) finish(end runEnd) {
	r.done, r.awaitAck = true, false
	if end.err != nil {
		r.push(outLine{"error: " + end.err.Error()})
		return
	}
	r.push(outLine{runFinishedText})
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
	case r.awaitAck:
		return runAckText
	case r.done:
		return runCloseText
	}
	return ""
}

// view re-wraps every line each frame; shortcut: fine at 2000 lines, cache wrapped lines if it shows in profiling.
func (r *run) view(th theme, w, h int) string {
	var lines []string
	for _, l := range r.lines {
		lines = append(lines, styled(th.plain, wrap(l.text, contentWidth(w)))...)
	}
	if r.follow {
		r.scroll.offset = len(lines)
	}
	inner := innerHeight(h)
	lines = r.scroll.window(lines, inner-1)
	for len(lines) < inner-1 {
		lines = append(lines, "")
	}
	lines = append(lines, th.muted.Render(r.hint()))
	return screenBox(th, runTitlePrefix+sanitizeLine(r.title), lines, w, h)
}
