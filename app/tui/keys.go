package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds the application-level key bindings. Navigation within the list
// and viewport (up/down, page up/down) is handled by those components' own
// default bindings; these are the bindings the top-level model interprets.
type keyMap struct {
	Enter key.Binding
	Back  key.Binding
	Quit  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "expand"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}
