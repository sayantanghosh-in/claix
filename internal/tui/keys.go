package tui

import "github.com/charmbracelet/bubbles/key"

// key.Binding defines a keyboard shortcut. Each binding has:
//   - keys: which keys trigger it (e.g., "enter", "q", "ctrl+c")
//   - help: what shows up in the help footer (key label + description)
//
// This is like defining keyboard shortcuts in VS Code's keybindings.json,
// but in code. The help bubble can automatically list all bindings.

// keyMap holds all the keyboard bindings for the TUI.
// We define them as a struct so we can pass them around and
// the help bubble can iterate over them.
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Quit   key.Binding
	Help   key.Binding
	Search key.Binding
	Tag    key.Binding
	Note   key.Binding
}

// keys is the global instance of our key bindings.
// var = package-level variable, accessible from other files in the same package.
var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),            // Arrow up or vim-style 'k'
		key.WithHelp("↑/k", "move up"),     // What shows in help footer
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),          // Arrow down or vim-style 'j'
		key.WithHelp("↓/j", "move down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "resume session"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Tag: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "add tag"),
	),
	Note: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "add note"),
	),
}
