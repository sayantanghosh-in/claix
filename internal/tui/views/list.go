// Package views will contain individual TUI view models as claix grows.
//
// Currently, the list view is built directly into the root app model (app.go)
// because claix only has one view. As we add detail view, help overlay, and
// search, each will become its own model in this package.
//
// The pattern will be:
//   - Each view implements Init(), Update(), View() (like a mini Bubbletea app)
//   - The root model in app.go switches between views using a state enum
//   - Views communicate with the parent via custom messages (like events in React)
//
// TODO: Extract list rendering from app.go into this file when adding more views.
package views
