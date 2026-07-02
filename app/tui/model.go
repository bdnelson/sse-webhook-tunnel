// Package tui implements the terminal user interface: a scrollable, paginated
// list of inbound events that can be expanded to inspect the JSON payload,
// with a status line showing uptime, event count, and the target URL.
package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bdnelson/sse-webhook-tunnel/core/event"
)

// EventMsg carries a completed event into the Bubble Tea message loop. It is
// sent by the Publisher from the tunnel goroutine.
type EventMsg struct {
	Event event.Event
}

// tickMsg drives the uptime clock in the status line.
type tickMsg time.Time

// viewMode is the current screen.
type viewMode int

const (
	listView viewMode = iota
	detailView
)

const (
	statusHeight = 1
	headerHeight = 2
)

// Model is the root Bubble Tea model.
type Model struct {
	list      list.Model
	viewport  viewport.Model
	keys      keyMap
	mode      viewMode
	startTime time.Time
	targetURL string
	sourceURL string
	count     int
	width     int
	height    int
	ready     bool
	now       func() time.Time

	// commandMode is true while the user is typing a ":" command.
	commandMode bool
	// command is the text typed after the ":" (excluding the colon).
	command string

	statusStyle  lipgloss.Style
	headerStyle  lipgloss.Style
	commandStyle lipgloss.Style
}

var _ tea.Model = Model{}

// New constructs the root model for the given source and target URLs.
func New(sourceURL, targetURL string) Model {
	l := list.New(nil, newItemDelegate(), 0, 0)
	l.Title = "SSE Webhook Tunnel — Events"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	// The list's own "q"/"esc" quit bindings are removed so it does not
	// advertise or consume them; quitting is handled by the model via ":q".
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "expand")),
			key.NewBinding(key.WithKeys(":"), key.WithHelp(":q", "quit")),
		}
	}

	return Model{
		list:      l,
		viewport:  viewport.New(),
		keys:      defaultKeyMap(),
		mode:      listView,
		startTime: time.Now(),
		targetURL: targetURL,
		sourceURL: sourceURL,
		now:       time.Now,
		statusStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("57")).
			Padding(0, 1),
		headerStyle:  lipgloss.NewStyle().Bold(true).Padding(0, 1),
		commandStyle: lipgloss.NewStyle().Padding(0, 1),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tick()
}

// tick schedules the next uptime refresh.
func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg), nil

	case tickMsg:
		return m, tick()

	case EventMsg:
		return m.handleEvent(msg), nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m.delegateToActive(msg)
}

// handleResize recomputes component dimensions to fit the terminal.
func (m Model) handleResize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true

	m.list.SetSize(msg.Width, msg.Height-statusHeight)
	m.viewport.SetWidth(msg.Width)
	m.viewport.SetHeight(msg.Height - headerHeight - statusHeight)
	return m
}

// handleEvent appends a received event to the list.
func (m Model) handleEvent(msg EventMsg) Model {
	m.count++
	items := m.list.Items()
	m.list.InsertItem(len(items), eventItem{ev: msg.Event})
	return m
}

// handleKey processes application-level key bindings and otherwise delegates to
// the active component.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// ctrl+c is always an emergency escape hatch.
	if key.Matches(msg, m.keys.ForceQuit) {
		return m, tea.Quit
	}

	// While typing a ":" command, keys edit the command line rather than
	// navigating.
	if m.commandMode {
		return m.handleCommandKey(msg)
	}

	// ":" opens the command line (from either view).
	if key.Matches(msg, m.keys.Command) {
		m.commandMode = true
		m.command = ""
		return m, nil
	}

	switch m.mode {
	case listView:
		if key.Matches(msg, m.keys.Enter) {
			if selected, ok := m.list.SelectedItem().(eventItem); ok {
				m.viewport.SetContent(selected.ev.PrettyJSON())
				m.viewport.GotoTop()
				m.mode = detailView
			}
			return m, nil
		}
	case detailView:
		if key.Matches(msg, m.keys.Back) {
			m.mode = listView
			return m, nil
		}
	}

	return m.delegateToActive(msg)
}

// handleCommandKey edits and executes the ":" command line. The command line is
// dismissed on Esc, on Enter, or when Backspace deletes past the colon. Only
// "q" (and "q!") quit; any other command is ignored.
func (m Model) handleCommandKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		command := m.command
		m.commandMode = false
		m.command = ""
		if command == "q" || command == "q!" {
			return m, tea.Quit
		}
		return m, nil
	case "esc":
		m.commandMode = false
		m.command = ""
		return m, nil
	case "backspace":
		if m.command == "" {
			m.commandMode = false
			return m, nil
		}
		runes := []rune(m.command)
		m.command = string(runes[:len(runes)-1])
		return m, nil
	default:
		// msg.Text holds the literal printable text (including space) for
		// text-producing keys, and is empty for control keys.
		if msg.Text != "" {
			m.command += msg.Text
		}
		return m, nil
	}
}

// delegateToActive forwards a message to whichever component is on screen so
// its navigation (cursor movement, paging, scrolling) works.
func (m Model) delegateToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.mode {
	case detailView:
		m.viewport, cmd = m.viewport.Update(msg)
	default:
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

// View implements tea.Model. In Bubble Tea v2 the view is a tea.View struct;
// AltScreen keeps the program in the alternate screen buffer (the v1
// WithAltScreen program option was removed).
func (m Model) View() tea.View {
	var content string
	switch {
	case !m.ready:
		content = "Initializing..."
	case m.mode == detailView:
		content = m.detailScreen()
	default:
		content = m.listScreen()
	}

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m Model) listScreen() string {
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), m.bottomLine())
}

func (m Model) detailScreen() string {
	var header string
	if selected, ok := m.list.SelectedItem().(eventItem); ok {
		header = m.headerStyle.Render(selected.ev.Summary() + "  (esc: back)")
	} else {
		header = m.headerStyle.Render("(esc: back)")
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View(), m.bottomLine())
}

// bottomLine renders the command line while a ":" command is being typed,
// otherwise the status line. Both occupy a single row so the layout is stable.
func (m Model) bottomLine() string {
	if m.commandMode {
		return m.commandLine()
	}
	return m.statusLine()
}

// commandLine renders the ":" command prompt with the current input.
func (m Model) commandLine() string {
	content := ":" + m.command
	if m.width > 0 {
		return m.commandStyle.Width(m.width).Render(content)
	}
	return m.commandStyle.Render(content)
}

// statusLine renders uptime, event count, and target URL.
func (m Model) statusLine() string {
	uptime := m.now().Sub(m.startTime).Truncate(time.Second)
	content := fmt.Sprintf("uptime %s │ events: %d │ target: %s",
		formatDuration(uptime), m.count, m.targetURL)

	line := m.statusStyle.Render(content)
	if m.width > 0 {
		line = m.statusStyle.Width(m.width).Render(content)
	}
	return line
}

// formatDuration renders a duration as HH:MM:SS.
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	h := total / 3600
	mn := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, mn, s)
}
