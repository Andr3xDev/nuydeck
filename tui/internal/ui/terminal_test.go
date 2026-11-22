package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// senderFunc adapts a function to messageSender.
type senderFunc func(tea.Msg)

func (f senderFunc) Send(msg tea.Msg) { f(msg) }

func TestRootTurnsExecRequestIntoExecProcess(t *testing.T) {
	req := execRequestMsg{cmd: exec.Command("true"), done: make(chan error, 1)}

	_, cmd := NewRoot("", false, Deps{}).Update(req)

	if cmd == nil {
		t.Fatal("no command returned for execRequestMsg")
	}
	// Calling the Cmd only wraps the process; nothing is started.
	if got := fmt.Sprintf("%T", cmd()); !strings.Contains(got, "exec") {
		t.Errorf("command produced %s, want the ExecProcess message", got)
	}
}

func TestExecCallbackReportsFailureAndReturnsControl(t *testing.T) {
	failure := errors.New("exit status 1")
	var sent execRequestMsg
	bridge := NewTerminalBridge()
	bridge.Attach(senderFunc(func(msg tea.Msg) {
		sent = msg.(execRequestMsg)
		if got := sent.finish(failure); got != (execFinishedMsg{err: failure}) {
			t.Errorf("callback returned %v, want the finished message", got)
		}
	}))

	if err := bridge.Run(context.Background(), exec.Command("sudo", "-v")); !errors.Is(err, failure) {
		t.Fatalf("Run = %v, want the callback error", err)
	}
	if cap(sent.done) != 1 {
		t.Errorf("done capacity = %d, want 1 so the callback never blocks", cap(sent.done))
	}
}

// stillRunningWindow is how long a Run that must keep waiting is observed before the test lets it finish.
const stillRunningWindow = 100 * time.Millisecond

func TestBridgeCancelWhileChildRunsWaitsForDone(t *testing.T) {
	childErr := errors.New("exit status 1")
	sent := make(chan execRequestMsg, 1)
	bridge := NewTerminalBridge()
	bridge.Attach(senderFunc(func(msg tea.Msg) { sent <- msg.(execRequestMsg) }))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- bridge.Run(ctx, exec.Command("sudo", "-v")) }()
	req := <-sent

	cancel()

	select {
	case err := <-result:
		t.Fatalf("Run returned %v while the child still owns the terminal", err)
	case <-time.After(stillRunningWindow):
	}
	req.finish(childErr)
	select {
	case err := <-result:
		if !errors.Is(err, childErr) {
			t.Errorf("Run = %v, want the child outcome", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Run stayed blocked after done fired")
	}
}

func TestBridgeDetachUnblocksAWaitingRun(t *testing.T) {
	bridge := NewTerminalBridge()
	bridge.Attach(senderFunc(func(tea.Msg) {}))
	result := make(chan error, 1)
	go func() { result <- bridge.Run(context.Background(), exec.Command("sudo", "-v")) }()
	time.Sleep(stillRunningWindow)

	bridge.Detach()

	select {
	case err := <-result:
		if !errors.Is(err, errBridgeDetached) {
			t.Errorf("Run = %v, want errBridgeDetached", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Run stayed blocked after Detach")
	}
}

func TestBridgeRunAfterDetachFailsWithoutBlocking(t *testing.T) {
	bridge := NewTerminalBridge()
	bridge.Attach(senderFunc(func(tea.Msg) {}))
	bridge.Detach()
	result := make(chan error, 1)

	go func() { result <- bridge.Run(context.Background(), exec.Command("sudo", "-v")) }()

	select {
	case err := <-result:
		if !errors.Is(err, errBridgeDetached) {
			t.Errorf("Run = %v, want errBridgeDetached", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Run blocked on a detached bridge")
	}
}

func TestBridgeCancelledBeforeRequestSendsNothing(t *testing.T) {
	bridge := NewTerminalBridge()
	bridge.Attach(senderFunc(func(tea.Msg) { t.Error("request sent after cancel") }))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := bridge.Run(ctx, exec.Command("sudo", "-v")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run = %v, want context.Canceled", err)
	}
}
