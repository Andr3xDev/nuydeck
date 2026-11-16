package ui

import (
	"context"
	"errors"
	"os/exec"

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
type TerminalBridge struct{ sender messageSender }

func NewTerminalBridge() *TerminalBridge { return &TerminalBridge{} }

// Attach must run before the program starts.
func (b *TerminalBridge) Attach(sender messageSender) { b.sender = sender }

// Run suspends the TUI while cmd owns the terminal. While the child runs no key reaches Root, so ctx can only
// be cancelled from outside; Run then returns without waiting for the child.
func (b *TerminalBridge) Run(ctx context.Context, cmd *exec.Cmd) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.sender == nil {
		return errBridgeDetached
	}
	done := make(chan error, 1)
	b.sender.Send(execRequestMsg{cmd: cmd, done: done})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
