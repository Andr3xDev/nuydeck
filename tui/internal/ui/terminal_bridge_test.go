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

func TestBridgeCancelUnblocksWhenTheRequestIsDropped(t *testing.T) {
	bridge := NewTerminalBridge()
	bridge.Attach(senderFunc(func(tea.Msg) {}))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- bridge.Run(ctx, exec.Command("sudo", "-v")) }()

	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run = %v, want context.Canceled", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Run stayed blocked after the context was cancelled")
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
