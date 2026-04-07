package scanner

// Scanner reads Claude Code session data from ~/.claude/projects/.
//
// TODO:
// - Walk ~/.claude/projects/ directory tree
// - Parse session files (JSON) to extract:
//   - Session ID
//   - Project path
//   - Timestamps (created, last active)
//   - Conversation messages (for summary extraction)
// - Return []Session sorted by last active
// - Handle missing/corrupt files gracefully

// Session represents a single Claude Code session with metadata.
type Session struct {
	ID          string
	ProjectPath string
	CreatedAt   int64
	LastActive  int64
	Summary     string
	Tags        []string
	Notes       string
	GitBranch   string
}
