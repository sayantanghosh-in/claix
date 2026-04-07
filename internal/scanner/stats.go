package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// =====================================================================
// STATS CACHE — Claude Code's aggregate usage data
// =====================================================================
//
// Claude Code maintains a stats-cache.json file at ~/.claude/stats-cache.json.
// It contains daily aggregate data:
//   - Messages sent per day
//   - Sessions started per day
//   - Tool calls made per day
//   - Token usage broken down by model (Opus, Sonnet, Haiku)
//
// We read this file to power the dashboard header in the TUI —
// activity sparkline, total stats, and token usage breakdown.

// Stats holds the parsed data from stats-cache.json.
type Stats struct {
	DailyActivity  []DailyActivity  // Messages, sessions, tool calls per day
	DailyTokens    []DailyTokens    // Token usage per day, broken down by model
	TotalMessages  int              // Sum of all message counts
	TotalSessions  int              // Sum of all session counts
	TotalToolCalls int              // Sum of all tool call counts
	TotalTokens    map[string]int   // Total tokens per model (e.g., {"claude-opus-4-6": 12400})
}

// DailyActivity represents one day's aggregate activity data.
type DailyActivity struct {
	Date          string `json:"date"`          // "2026-03-11" (YYYY-MM-DD format)
	MessageCount  int    `json:"messageCount"`  // Total messages exchanged
	SessionCount  int    `json:"sessionCount"`  // Number of sessions started
	ToolCallCount int    `json:"toolCallCount"` // Number of tool calls made by Claude
}

// DailyTokens represents one day's token usage, broken down by model.
type DailyTokens struct {
	Date          string         `json:"date"`          // "2026-03-11"
	TokensByModel map[string]int `json:"tokensByModel"` // e.g., {"claude-opus-4-6": 301, "claude-sonnet-4-6": 1147}
}

// statsCache is the raw JSON structure of ~/.claude/stats-cache.json.
// We use this intermediate struct to unmarshal the file, then convert to Stats.
type statsCache struct {
	Version          int             `json:"version"`
	LastComputedDate string          `json:"lastComputedDate"`
	DailyActivity    []DailyActivity `json:"dailyActivity"`
	DailyModelTokens []DailyTokens  `json:"dailyModelTokens"`
}

// LoadStats reads and parses the Claude Code stats cache file.
// Pass "" for claudeHome to use the default ~/.claude.
func LoadStats(claudeHome string) (*Stats, error) {
	if claudeHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		claudeHome = filepath.Join(homeDir, ".claude")
	}

	statsPath := filepath.Join(claudeHome, "stats-cache.json")

	// os.ReadFile reads the entire file into memory — fine here since stats-cache.json
	// is small (a few KB). This is simpler than streaming for small files.
	data, err := os.ReadFile(statsPath)
	if err != nil {
		// If the file doesn't exist, return empty stats (not an error).
		// The user might be new to Claude Code or the cache hasn't been created yet.
		if os.IsNotExist(err) {
			return &Stats{TotalTokens: make(map[string]int)}, nil
		}
		return nil, err
	}

	// Parse the JSON into our intermediate struct
	var cache statsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	// Calculate totals by summing across all days
	stats := &Stats{
		DailyActivity: cache.DailyActivity,
		DailyTokens:   cache.DailyModelTokens,
		TotalTokens:   make(map[string]int),
	}

	for _, day := range cache.DailyActivity {
		stats.TotalMessages += day.MessageCount
		stats.TotalSessions += day.SessionCount
		stats.TotalToolCalls += day.ToolCallCount
	}

	for _, day := range cache.DailyModelTokens {
		for model, tokens := range day.TokensByModel {
			stats.TotalTokens[model] += tokens
		}
	}

	return stats, nil
}

// SparklineData returns the last N days of message counts as an array of ints,
// suitable for rendering as a sparkline (bar chart using Unicode block characters).
//
// It fills in zeros for days with no activity so the sparkline is continuous.
// This is like generating x-axis data points for a time series chart.
func (s *Stats) SparklineData(days int) []int {
	if len(s.DailyActivity) == 0 {
		return make([]int, days) // Return all zeros
	}

	// Sort by date to ensure chronological order
	sorted := make([]DailyActivity, len(s.DailyActivity))
	copy(sorted, s.DailyActivity)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date < sorted[j].Date
	})

	// Build a map of date → message count for quick lookup
	// map[string]int is like a Record<string, number> in TypeScript
	dateMap := make(map[string]int)
	for _, day := range sorted {
		dateMap[day.Date] = day.MessageCount
	}

	// Get the last N days worth of data.
	// We use the last date in the data as the end point, then work backwards.
	lastDate := sorted[len(sorted)-1].Date

	// Parse the last date to generate previous dates.
	// We'll use a simple approach: generate date strings by subtracting days.
	result := make([]int, days)
	for i := days - 1; i >= 0; i-- {
		if count, ok := dateMap[lastDate]; ok {
			result[i] = count
		}
		lastDate = prevDate(lastDate)
	}

	return result
}

// prevDate subtracts one day from a "YYYY-MM-DD" date string.
// This is a simple implementation that handles month/year boundaries.
func prevDate(date string) string {
	// Parse year, month, day from the string
	// We avoid importing time package for this simple operation.
	if len(date) != 10 {
		return date
	}

	year := parseInt(date[0:4])
	month := parseInt(date[5:7])
	day := parseInt(date[8:10])

	day--
	if day < 1 {
		month--
		if month < 1 {
			month = 12
			year--
		}
		// Days in each month (non-leap year approximation — close enough for a sparkline)
		daysInMonth := []int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
		if month >= 1 && month <= 12 {
			day = daysInMonth[month]
		} else {
			day = 30
		}
	}

	return formatDate(year, month, day)
}

// parseInt converts a string like "03" to int 3. Returns 0 on error.
func parseInt(s string) int {
	n := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

// formatDate returns "YYYY-MM-DD" from individual components.
func formatDate(year, month, day int) string {
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

// RenderSparkline converts an array of ints to a Unicode sparkline string.
// Each value is mapped to one of 8 bar heights: ▁▂▃▄▅▆▇█
//
// Example: [0, 5, 10, 3, 8, 0, 2] → "▁▃█▂▆▁▂"
func RenderSparkline(data []int) string {
	if len(data) == 0 {
		return ""
	}

	// Find the max value to normalize the bars
	max := 0
	for _, v := range data {
		if v > max {
			max = v
		}
	}

	if max == 0 {
		// All zeros — return flat line
		result := make([]rune, len(data))
		for i := range result {
			result[i] = '▁'
		}
		return string(result)
	}

	// The 8 Unicode block characters, from shortest to tallest.
	// These are standard Unicode chars that render in most terminal fonts.
	bars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	result := make([]rune, len(data))
	for i, v := range data {
		// Normalize the value to a 0-7 index.
		// (v * 7 / max) maps the value to one of 8 bar heights.
		idx := v * 7 / max
		if idx > 7 {
			idx = 7
		}
		result[i] = bars[idx]
	}

	return string(result)
}

// FormatTokenCount formats a large number with k/M suffix.
// 1234 → "1.2k", 1234567 → "1.2M", 500 → "500"
func FormatTokenCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// ModelShortName converts a full model ID to a short display name.
// "claude-opus-4-6" → "opus"
// "claude-sonnet-4-6" → "sonnet"
// "claude-haiku-4-5-20251001" → "haiku"
func ModelShortName(model string) string {
	lower := model
	switch {
	case contains(lower, "opus"):
		return "opus"
	case contains(lower, "sonnet"):
		return "sonnet"
	case contains(lower, "haiku"):
		return "haiku"
	default:
		return model
	}
}

// contains checks if s contains substr (case-insensitive isn't needed here
// since model IDs are always lowercase).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
