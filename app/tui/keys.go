package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds the application-level key bindings. Navigation within the list
// and viewport (up/down, page up/down) is handled by those components' own
// default bindings; these are the bindings the top-level model interprets.
//
// Quitting is deliberate: the user opens a command line with ":" and runs "q"
// (vim style), so a stray keystroke cannot end the session. ForceQuit (ctrl+c)
// remains as an emergency escape hatch.
type keyMap struct {
	Enter     key.Binding
	Back      key.Binding
	Command   key.Binding
	ForceQuit key.Binding
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
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
	}
}
