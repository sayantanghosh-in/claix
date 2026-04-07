package tui

// App is the root Bubbletea model that multiplexes between views.
// Each view (list, detail, help) is a separate model implementing tea.Model.
//
// TODO:
// - Initialize bubbletea program with alt screen
// - Handle global keybindings (quit, help toggle, view switching)
// - Propagate WindowSizeMsg to child views
// - Route messages to the active view
