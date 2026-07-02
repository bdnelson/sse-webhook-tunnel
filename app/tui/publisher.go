package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/bdnelson/sse-webhook-tunnel/core/event"
)

// sender is the subset of *tea.Program the publisher depends on.
type sender interface {
	Send(tea.Msg)
}

// Publisher adapts a Bubble Tea program to the tunnel's EventPublisher
// interface, delivering events into the UI message loop.
type Publisher struct {
	program sender
}

// NewPublisher wraps a Bubble Tea program.
func NewPublisher(program sender) Publisher {
	return Publisher{program: program}
}

// Publish sends the event into the UI as an EventMsg.
func (p Publisher) Publish(e event.Event) {
	p.program.Send(EventMsg{Event: e})
}
