package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bdnelson/sse-webhook-tunnel/core/event"
)

// newTestModel returns a resized, ready model.
func newTestModel(t *testing.T) Model {
	t.Helper()
	m := New("https://smee.io/abc", "http://localhost:9000/hook")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(Model)
}

func sampleEvent(sec int) event.Event {
	return event.Event{
		Time:      time.Date(2026, 7, 2, 13, 35, sec, 0, time.UTC),
		Raw:       []byte(`{"body":{"action":"opened"}}`),
		Forwarded: true,
		Status:    200,
	}
}

func TestModel_HandleResize_SetsDimensions(t *testing.T) {
	m := newTestModel(t)
	if !m.ready {
		t.Fatal("model should be ready after resize")
	}
	if m.width != 80 || m.height != 24 {
		t.Errorf("dimensions = %dx%d, want 80x24", m.width, m.height)
	}
	if m.viewport.Height != 24-headerHeight-statusHeight {
		t.Errorf("viewport height = %d, want %d", m.viewport.Height, 24-headerHeight-statusHeight)
	}
}

func TestModel_EventMsg_AppendsAndCounts(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(EventMsg{Event: sampleEvent(52)})
	m = updated.(Model)
	updated, _ = m.Update(EventMsg{Event: sampleEvent(53)})
	m = updated.(Model)

	if m.count != 2 {
		t.Errorf("count = %d, want 2", m.count)
	}
	if len(m.list.Items()) != 2 {
		t.Errorf("list items = %d, want 2", len(m.list.Items()))
	}
}

func TestModel_Enter_ExpandsToDetail(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(EventMsg{Event: sampleEvent(52)})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.mode != detailView {
		t.Fatalf("mode = %v, want detailView", m.mode)
	}
	// The detail screen should contain the pretty-printed payload.
	if !strings.Contains(m.View(), "action") {
		t.Errorf("detail view missing payload content:\n%s", m.View())
	}
}

func TestModel_Enter_NoItemsStaysInList(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != listView {
		t.Errorf("mode = %v, want listView when no items", m.mode)
	}
}

func TestModel_Esc_ReturnsToList(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(EventMsg{Event: sampleEvent(52)})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != detailView {
		t.Fatal("precondition: should be in detail view")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != listView {
		t.Errorf("mode = %v, want listView after esc", m.mode)
	}
}

func TestModel_Quit(t *testing.T) {
	m := newTestModel(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if msg := cmd(); msg == nil {
		t.Error("quit command produced nil message")
	}
	// ctrl+c should also quit.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command for ctrl+c")
	}
}

func TestModel_StatusLine_ShowsUptimeCountTarget(t *testing.T) {
	fixedStart := time.Date(2026, 7, 2, 13, 0, 0, 0, time.UTC)
	m := newTestModel(t)
	m.startTime = fixedStart
	m.now = func() time.Time { return fixedStart.Add(65 * time.Second) }

	updated, _ := m.Update(EventMsg{Event: sampleEvent(52)})
	m = updated.(Model)

	status := m.statusLine()
	for _, want := range []string{"00:01:05", "events: 1", "http://localhost:9000/hook"} {
		if !strings.Contains(status, want) {
			t.Errorf("status line missing %q; got:\n%s", want, status)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00:00"},
		{65 * time.Second, "00:01:05"},
		{3661 * time.Second, "01:01:01"},
		{-5 * time.Second, "00:00:00"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.d); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
