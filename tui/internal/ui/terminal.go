package ui

import (
	"context"
	"errors"
	"os/exec"
	"sync"

	tea "charm.land/bubbletea/v2"
)

var errBridgeDetached = errors.New("terminal bridge is not attached to a program")

// messageSender is the part of *tea.Program the bridge needs.
type messageSender interface{ Send(tea.Msg) }

// execRequestMsg asks Root to hand the terminal to cmd; the outcome goes to done.
type execRequestMsg struct {
	cmd  *exec.Cmd
	done chan error
}

type execFinishedMsg struct{ err error }

// finish is the tea.ExecProcess callback: it reports the outcome and always returns control to the program.
func (r execRequestMsg) finish(err error) tea.Msg {
	r.done <- err
	return execFinishedMsg{err: err}
}

// TerminalBridge implements plan.Terminal on top of tea.ExecProcess; cmd/nuydeck attaches the program before Run.
type TerminalBridge struct {
	sender     messageSender
	closed     chan struct{}
	detachOnce sync.Once
}

func NewTerminalBridge() *TerminalBridge { return &TerminalBridge{closed: make(chan struct{})} }

// Attach must run before the program starts.
func (b *TerminalBridge) Attach(sender messageSender) { b.sender = sender }

// Detach releases every waiting Run and makes later ones fail; cmd/nuydeck calls it once the program has exited.
func (b *TerminalBridge) Detach() { b.detachOnce.Do(func() { close(b.closed) }) }

func (b *TerminalBridge) detached() bool {
	select {
	case <-b.closed:
		return true
	default:
		return false
	}
}

// Run suspends the TUI while cmd owns the terminal. Once the request is sent it waits for the child even if ctx is
// cancelled: returning earlier would let the caller write to a terminal the child still owns.
func (b *TerminalBridge) Run(ctx context.Context, cmd *exec.Cmd) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.sender == nil || b.detached() {
		return errBridgeDetached
	}
	done := make(chan error, 1)
	b.sender.Send(execRequestMsg{cmd: cmd, done: done})
	select {
	case err := <-done:
		return err
	case <-b.closed:
		return errBridgeDetached
	}
}
