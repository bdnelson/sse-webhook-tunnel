package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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
	if m.viewport.Height() != 24-headerHeight-statusHeight {
		t.Errorf("viewport height = %d, want %d", m.viewport.Height(), 24-headerHeight-statusHeight)
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

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.mode != detailView {
		t.Fatalf("mode = %v, want detailView", m.mode)
	}
	// The detail screen should contain the pretty-printed payload.
	if !strings.Contains(m.View().Content, "action") {
		t.Errorf("detail view missing payload content:\n%s", m.View().Content)
	}
}

func TestModel_Enter_NoItemsStaysInList(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != listView {
		t.Errorf("mode = %v, want listView when no items", m.mode)
	}
}

func TestModel_Esc_ReturnsToList(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(EventMsg{Event: sampleEvent(52)})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != detailView {
		t.Fatal("precondition: should be in detail view")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != listView {
		t.Errorf("mode = %v, want listView after esc", m.mode)
	}
}

// pressRunes feeds each rune to the model as a key press, setting Text so the
// model's String()/Text-based handling sees printable input.
func pressRunes(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		updated, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}
	return m
}

func TestModel_BareQ_DoesNotQuit(t *testing.T) {
	m := newTestModel(t)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		t.Errorf("bare 'q' should not quit, got command %v", cmd())
	}
}

func TestModel_ColonQEnter_Quits(t *testing.T) {
	m := newTestModel(t)

	// ":" opens the command line.
	updated, _ := m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	m = updated.(Model)
	if !m.commandMode {
		t.Fatal("expected command mode after ':'")
	}

	// Typing "q" should not quit until Enter.
	m = pressRunes(t, m, "q")
	if m.command != "q" {
		t.Errorf("command buffer = %q, want \"q\"", m.command)
	}

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected quit command after ':q<enter>'")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
	if m.commandMode {
		t.Error("command mode should be cleared after execution")
	}
}

func TestModel_CtrlC_ForceQuits(t *testing.T) {
	m := newTestModel(t)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected force-quit command for ctrl+c")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestModel_CommandLine_EscCancels(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	m = updated.(Model)
	m = pressRunes(t, m, "q")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)
	if m.commandMode {
		t.Error("esc should cancel command mode")
	}
	if cmd != nil {
		t.Errorf("esc should not quit, got %v", cmd())
	}
}

func TestModel_CommandLine_UnknownCommandIgnored(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	m = updated.(Model)
	m = pressRunes(t, m, "xyz")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil {
		t.Errorf("unknown command should not quit, got %v", cmd())
	}
	if m.commandMode {
		t.Error("command mode should be cleared after unknown command")
	}
}

func TestModel_CommandLine_Backspace(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	m = updated.(Model)
	m = pressRunes(t, m, "q")

	// Backspace clears the buffer; a second backspace closes the command line.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = updated.(Model)
	if m.command != "" {
		t.Errorf("command buffer = %q, want empty", m.command)
	}
	if !m.commandMode {
		t.Error("command mode should remain until backspace past the colon")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = updated.(Model)
	if m.commandMode {
		t.Error("backspace past the colon should close the command line")
	}
}

func TestModel_CommandLine_RenderedInView(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	m = updated.(Model)
	m = pressRunes(t, m, "q")
	if !strings.Contains(m.View().Content, ":q") {
		t.Errorf("view should show the command line ':q'; got:\n%s", m.View().Content)
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
