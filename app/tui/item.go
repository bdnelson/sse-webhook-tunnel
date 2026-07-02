package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bdnelson/sse-webhook-tunnel/core/event"
)

// eventItem adapts an event.Event to the list.Item interface.
type eventItem struct {
	ev event.Event
}

var _ list.Item = eventItem{}

// FilterValue implements list.Item.
func (i eventItem) FilterValue() string { return i.ev.Summary() }

// itemDelegate renders each event as a single line: its timestamp summary plus
// a forwarding status indicator.
type itemDelegate struct {
	selected lipgloss.Style
	normal   lipgloss.Style
	ok       lipgloss.Style
	failed   lipgloss.Style
}

var _ list.ItemDelegate = itemDelegate{}

func newItemDelegate() itemDelegate {
	return itemDelegate{
		selected: lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true),
		normal:   lipgloss.NewStyle(),
		ok:       lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		failed:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	}
}

func (d itemDelegate) Height() int  { return 1 }
func (d itemDelegate) Spacing() int { return 0 }

func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render implements list.ItemDelegate.
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(eventItem)
	if !ok {
		return
	}

	var status string
	if it.ev.ForwardErr != nil {
		status = d.failed.Render("forward error")
	} else {
		status = d.ok.Render(fmt.Sprintf("%d", it.ev.Status))
	}

	line := fmt.Sprintf("%s  [%s]", it.ev.Summary(), status)

	if index == m.Index() {
		fmt.Fprint(w, d.selected.Render("> ")+line)
		return
	}
	fmt.Fprint(w, d.normal.Render("  ")+line)
}
