package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	runNotice       = "a run is in progress, ctrl+c cancels"
	settleTime      = 50 * time.Millisecond
	outputLineCap   = 2000
	overflowLines   = outputLineCap + 500
	shutdownTimeout = 10 * time.Second
	promptReturn    = time.Second
)

type testWork = func(ctx context.Context, emit func(plan.Event)) runEnd

// driver runs the commands Root returns the way the Bubble Tea runtime does, so work goroutines and their messages flow.
type driver struct {
	root *Root
	msgs chan tea.Msg
	done chan struct{}
	quit bool
}

func newDriver(t *testing.T, root *Root) *driver {
	t.Helper()
	d := &driver{root: root, msgs: make(chan tea.Msg), done: make(chan struct{})}
	t.Cleanup(func() {
		close(d.done)
		root.cancel()
	})
	return d
}

func (d *driver) launch(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		select {
		case d.msgs <- cmd():
		case <-d.done:
		}
	}()
}

func (d *driver) deliver(msg tea.Msg) {
	switch m := msg.(type) {
	case tea.BatchMsg:
		for _, c := range m {
			d.launch(c)
		}
	case tea.QuitMsg:
		d.quit = true
	default:
		_, cmd := d.root.Update(m)
		d.launch(cmd)
	}
}

// until feeds messages to Root until stop reports true; it reports false on timeout.
func (d *driver) until(limit time.Duration, stop func() bool) bool {
	deadline := time.After(limit)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for !stop() {
		select {
		case msg := <-d.msgs:
			d.deliver(msg)
		case <-tick.C:
		case <-deadline:
			return false
		}
	}
	return true
}

func (d *driver) must(t *testing.T, what string, stop func() bool) {
	t.Helper()
	if !d.until(waitTimeout, stop) {
		t.Fatalf("timed out waiting for %s", what)
	}
}

// settle lets pending messages land, so a key that must do nothing has had time to do something.
func (d *driver) settle() { d.until(settleTime, func() bool { return false }) }

func (d *driver) press(k tea.KeyPressMsg) {
	_, cmd := d.root.Update(k)
	d.launch(cmd)
}

func (d *driver) runDone() bool { return d.root.run != nil && d.root.run.done }

func (d *driver) view() string { return ansi.Strip(d.root.View().Content) }

// runningRoot sends a run request from the Install section and returns once the work goroutine has started.
func runningRoot(t *testing.T, dryRun bool, f focus, ack chan struct{}, work testWork) *driver {
	t.Helper()
	root := NewRoot("", dryRun, Deps{})
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_, root.selected, _ = sectionOf[*Install](root)
	root.focus = f
	d := newDriver(t, root)

	started := make(chan struct{})
	_, cmd := root.Update(runRequest{title: "test run", ack: ack, work: func(ctx context.Context, emit func(plan.Event)) runEnd {
		close(started)
		return work(ctx, emit)
	}})
	d.launch(cmd)
	d.must(t, "the work to start", func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	return d
}

// blockedUntilCancelled holds the run open and signals when the context is cancelled.
func blockedUntilCancelled(cancelled chan struct{}) testWork {
	return func(ctx context.Context, _ func(plan.Event)) runEnd {
		<-ctx.Done()
		cancelled <- struct{}{}
		return runEnd{}
	}
}

func finishesAtOnce(context.Context, func(plan.Event)) runEnd { return runEnd{} }

func TestRunRefusesEveryKeyButCtrlCWithANotice(t *testing.T) {
	refused := map[string]tea.KeyPressMsg{
		"q": keyPress('q'), "esc": keyEsc, "x": keyPress('x'), "tab": keyTab, "?": keyPress('?'), "enter": keyEnter,
	}
	for _, f := range []focus{focusSidebar, focusPanel} {
		for _, dryRun := range []bool{false, true} {
			for name, k := range refused {
				t.Run(fmt.Sprintf("focus=%d/dry-run=%v/%s", f, dryRun, name), func(t *testing.T) {
					cancelled := make(chan struct{}, 1)
					d := runningRoot(t, dryRun, f, nil, blockedUntilCancelled(cancelled))

					d.press(k)
					d.settle()

					if d.quit {
						t.Error("the program quit")
					}
					if d.root.run == nil || d.runDone() || len(cancelled) != 0 {
						t.Errorf("run open=%v done=%v cancelled=%v, want it still running", d.root.run != nil, d.runDone(), len(cancelled) != 0)
					}
					if d.root.focus != f || d.root.showHelp {
						t.Errorf("focus=%v help=%v, want the key to change nothing", d.root.focus, d.root.showHelp)
					}
					if !strings.Contains(d.view(), runNotice) {
						t.Errorf("view lacks the notice %q", runNotice)
					}
				})
			}
		}
	}
}

func TestRunCtrlCCancelsAndTheProgramStaysOpen(t *testing.T) {
	cancelled := make(chan struct{}, 1)
	d := runningRoot(t, false, focusSidebar, nil, blockedUntilCancelled(cancelled))

	d.press(keyCtrlC)

	if !receives(cancelled) {
		t.Fatal("ctrl+c did not cancel the run")
	}
	d.must(t, "the run to report done", d.runDone)
	d.settle()
	if d.quit || d.root.run == nil {
		t.Errorf("quit=%v pane open=%v, want the program open with the finished pane", d.quit, d.root.run != nil)
	}
}

func TestRunEscClosesAFinishedRunAndBroadcastsRunClosed(t *testing.T) {
	d := runningRoot(t, false, focusPanel, nil, finishesAtOnce)
	listener := &probeScreen{}
	d.root.sections = append(d.root.sections, section{title: "Listener", screen: listener})
	d.must(t, "the run to finish", d.runDone)

	d.press(keyEsc)
	d.settle()

	if d.root.run != nil {
		t.Error("esc did not close the run pane")
	}
	closed := slices.ContainsFunc(listener.broadcasts, func(m tea.Msg) bool { _, ok := m.(runClosed); return ok })
	if !closed {
		t.Errorf("screens got %v, want runClosed", listener.broadcasts)
	}
}

func TestRunQuitsFromAFinishedRun(t *testing.T) {
	for name, k := range map[string]tea.KeyPressMsg{"q": keyPress('q'), "ctrl+c": keyCtrlC} {
		t.Run(name, func(t *testing.T) {
			d := runningRoot(t, false, focusSidebar, nil, finishesAtOnce)
			d.must(t, "the run to finish", d.runDone)

			d.press(k)

			if !d.until(waitTimeout, func() bool { return d.quit }) {
				t.Errorf("%s did not quit a finished run", name)
			}
		})
	}
}

// awaitingReboot drives a run to the reboot prompt; the work reports how it ended.
func awaitingReboot(t *testing.T) (d *driver, acked, cancelled chan struct{}) {
	t.Helper()
	acked, cancelled = make(chan struct{}, 1), make(chan struct{}, 1)
	ack := make(chan struct{}, 1)
	d = runningRoot(t, false, focusPanel, ack, func(ctx context.Context, emit func(plan.Event)) runEnd {
		emit(plan.Event{Kind: plan.EventAwaitAck})
		select {
		case <-ack:
			acked <- struct{}{}
		case <-ctx.Done():
			cancelled <- struct{}{}
		}
		return runEnd{}
	})
	d.must(t, "the reboot prompt", func() bool { return d.root.run.awaitAck })
	return d, acked, cancelled
}

func TestRunRebootPromptAcceptsEnterOnly(t *testing.T) {
	d, acked, cancelled := awaitingReboot(t)

	for _, k := range []tea.KeyPressMsg{keyPress('q'), keyPress('x'), keyPress(tea.KeySpace), keyEsc} {
		d.press(k)
	}
	d.settle()
	if d.quit || len(acked) != 0 || len(cancelled) != 0 || !d.root.run.awaitAck {
		t.Fatalf("after other keys: quit=%v acked=%d cancelled=%d prompt=%v, want the prompt untouched", d.quit, len(acked), len(cancelled), d.root.run.awaitAck)
	}

	d.press(keyEnter)

	if !receives(acked) {
		t.Fatal("enter did not acknowledge the reboot")
	}
	if len(cancelled) != 0 {
		t.Error("the run was cancelled")
	}
}

func TestRunCtrlCAtTheRebootPromptQuitsWithoutRebooting(t *testing.T) {
	d, acked, cancelled := awaitingReboot(t)

	d.press(keyCtrlC)

	if !d.until(waitTimeout, func() bool { return d.quit }) {
		t.Fatal("ctrl+c did not quit")
	}
	if !receives(cancelled) {
		t.Error("ctrl+c did not cancel the run")
	}
	if len(acked) != 0 {
		t.Error("ctrl+c acknowledged the reboot")
	}
}

// R4: the old install handle was created inside the started command, so quitting before it ran blocked the shutdown wait.
func TestRunWaitReturnsAtOnceWhenTheWorkNeverStarted(t *testing.T) {
	root := NewRoot("", false, Deps{})
	root.Update(runRequest{title: "never started", work: func(context.Context, func(plan.Event)) runEnd {
		t.Error("the work ran although its command was never executed")
		return runEnd{}
	}})
	root.Update(keyCtrlC)

	begin := time.Now()
	root.WaitInstall(shutdownTimeout)

	if elapsed := time.Since(begin); elapsed > promptReturn {
		t.Errorf("WaitInstall took %v, want it to return at once", elapsed)
	}
}

func TestRunKeepsOnlyTheLastOutputLines(t *testing.T) {
	d := runningRoot(t, false, focusPanel, nil, func(_ context.Context, emit func(plan.Event)) runEnd {
		for i := range overflowLines {
			emit(plan.Event{Kind: plan.EventLine, Line: fmt.Sprintf("row-%04d", i)})
		}
		return runEnd{}
	})
	d.must(t, "the run to finish", d.runDone)

	lines := d.root.run.lines
	if len(lines) != outputLineCap {
		t.Fatalf("kept %d lines, want %d", len(lines), outputLineCap)
	}
	if strings.Contains(lines[0].text, "row-0000") {
		t.Error("the oldest line was kept instead of dropped")
	}
}

func TestRunShowsItsOutputInOnePane(t *testing.T) {
	d := runningRoot(t, false, focusPanel, nil, func(ctx context.Context, emit func(plan.Event)) runEnd {
		emit(plan.Event{Kind: plan.EventLine, Line: "marker-line"})
		<-ctx.Done()
		return runEnd{}
	})

	d.must(t, "the output on screen", func() bool { return strings.Contains(d.view(), "marker-line") })
}
