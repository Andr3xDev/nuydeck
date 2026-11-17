package ui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// screen is one sidebar section's panel.
type screen interface {
	Title() string
	// Enter runs when the section becomes selected.
	Enter() tea.Cmd
	Update(msg tea.Msg) (screen, tea.Cmd)
	View(w, h int) string
	// Keys lists the bindings of the screen's current stage: the shell delivers only these keys, and the keybar and help show them.
	Keys() []key.Binding
}

// initer is implemented by screens that start work when the program starts.
type initer interface{ Init() tea.Cmd }

// textCapturer is implemented by screens that may be reading typed text, so shell keys such as ? and q must reach them.
type textCapturer interface{ CapturesText() bool }

// leaver is implemented by screens that clean up when the selection moves off them.
type leaver interface{ Leave() }

// canceller is implemented by screens with background work to stop on quit.
type canceller interface{ Cancel() }

// services is what every screen gets from the shell.
type services struct {
	Deps
	ctx    context.Context
	dryRun bool
	th     theme
	keys   keyMap
	// running reports whether Root's output pane is open; nil means never (screens under test).
	running func() bool
}

func (s *services) runIsActive() bool { return s.running != nil && s.running() }
