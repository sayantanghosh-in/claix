package config

// Config manages claix configuration stored at ~/.config/claix/config.json.
//
// TODO:
// - Define Config struct (claude home path, theme, default sort, etc.)
// - Load from file with sensible defaults
// - Save config changes
// - Support XDG config dirs

// Config holds the claix configuration.
type Config struct {
	ClaudeHome string `json:"claude_home"`
	Theme      string `json:"theme"`
	SortBy     string `json:"sort_by"`
}
