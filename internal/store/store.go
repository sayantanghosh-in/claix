package store

// Store manages claix's own metadata (tags, notes, pins) persisted locally.
// This is separate from Claude's session files — we never modify those.
//
// TODO:
// - Define Store interface
// - Implement JSON file-based store at ~/.config/claix/store.json
// - CRUD operations for session metadata (tags, notes, pinned status)
// - Merge with scanner data at read time
