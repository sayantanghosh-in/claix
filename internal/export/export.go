// Package export generates markdown summaries of Claude Code sessions.
//
// Given a parsed Session (from the scanner package), it produces a structured
// markdown document with conversation highlights, file changes, and PR links.
// This is useful for saving session notes, sharing context with teammates,
// or reviewing what happened in a long conversation.
package export

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sayantanghosh-in/claix/internal/scanner"
)

// maxHighlights is the number of user/assistant message pairs to include
// in the "Conversation Highlights" section.
const maxHighlights = 5

// maxMessageLen is the maximum character length for each highlighted message.
// Messages longer than this are truncated with "..." appended.
const maxMessageLen = 200

// =====================================================================
// MAIN EXPORT FUNCTION
// =====================================================================

// Export takes a fully-parsed Session struct and returns a formatted markdown
// string summarizing the session. This is the primary function callers should use
// when they already have a Session object (e.g., from scanner.ScanAll).
func Export(session scanner.Session) string {
	var b strings.Builder

	// ── Header: session ID and metadata ──
	shortID := session.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	b.WriteString(fmt.Sprintf("## Session: %s\n", shortID))

	// Project path — use " > " separator to avoid terminal URL detection.
	// Convert "/Users/<user>/git/my-project" → "git > my-project"
	projectDisplay := shortenProjectPath(session.ProjectPath)
	b.WriteString(fmt.Sprintf("**Project:** %s  \n", projectDisplay))

	// Git branch
	branch := session.GitBranch
	if branch == "" {
		branch = "HEAD"
	}
	b.WriteString(fmt.Sprintf("**Branch:** %s  \n", branch))

	// Date — extract just "YYYY-MM-DD HH:MM" from the ISO timestamp
	date := formatDate(session.StartedAt)
	b.WriteString(fmt.Sprintf("**Date:** %s  \n", date))

	// Status
	status := "Done"
	switch session.Status {
	case scanner.StatusActive:
		status = "Active"
	case scanner.StatusEmpty:
		status = "Empty"
	}
	b.WriteString(fmt.Sprintf("**Status:** %s  \n", status))

	// Message counts
	b.WriteString(fmt.Sprintf("**Messages:** %d (%d you, %d Claude)\n",
		session.TotalMsgs, session.UserMsgs, session.AssistantMsgs))

	// ── Summary (title) ──
	if session.Title != "" {
		b.WriteString(fmt.Sprintf("\n### Summary\n%s\n", session.Title))
	}

	// ── Conversation Highlights ──
	// We read the .jsonl file again to extract the first N user/assistant pairs.
	// The Session struct has the file path, so we can access the raw data.
	highlights := extractHighlights(session.FilePath, maxHighlights)
	if len(highlights) > 0 {
		b.WriteString("\n### Conversation Highlights\n")
		for _, h := range highlights {
			b.WriteString(fmt.Sprintf("> **You:** %s\n", h.userMsg))
			b.WriteString(fmt.Sprintf("> **Claude:** %s\n\n", h.assistantMsg))
		}
	}

	// ── Files Changed ──
	readCount := len(session.Activity.FilesRead)
	editCount := len(session.Activity.FilesEdited)
	if readCount > 0 || editCount > 0 {
		b.WriteString("### Files Changed\n")
		if readCount > 0 {
			b.WriteString(fmt.Sprintf("- **Read:** %d files\n", readCount))
		}
		if editCount > 0 {
			b.WriteString(fmt.Sprintf("- **Edited:** %d files\n", editCount))
		}
		if session.Activity.RepoSummary != "" {
			b.WriteString(fmt.Sprintf("- Repos: %s\n", session.Activity.RepoSummary))
		}
	}

	// ── Pull Requests ──
	if len(session.PRLinks) > 0 {
		b.WriteString("\n### Pull Requests\n")
		for _, pr := range session.PRLinks {
			b.WriteString(fmt.Sprintf("- PR #%d: %s\n", pr.Number, pr.URL))
		}
	}

	return b.String()
}

// =====================================================================
// EXPORT FROM FILE (convenience function)
// =====================================================================

// ExportFromFile is a convenience function that parses a .jsonl file path
// into a Session and then generates the markdown export. Use this when you
// have a file path but not a pre-parsed Session.
//
// Note: This function creates a minimal Session by scanning the file.
// For full metadata (project path, etc.), prefer using Export() with a
// Session obtained from scanner.ScanAll().
func ExportFromFile(jsonlPath string) (string, error) {
	// We need to scan all sessions and find the one matching this file path,
	// because parseSession is unexported. Instead, we scan all and match.
	sessions, err := scanner.ScanAll("")
	if err != nil {
		return "", fmt.Errorf("failed to scan sessions: %w", err)
	}

	for _, s := range sessions {
		if s.FilePath == jsonlPath {
			return Export(s), nil
		}
	}

	return "", fmt.Errorf("session not found for file: %s", jsonlPath)
}

// =====================================================================
// CONVERSATION HIGHLIGHTS
// =====================================================================

// highlight represents a single user/assistant message pair
// for the "Conversation Highlights" section of the export.
type highlight struct {
	userMsg      string // Truncated user message
	assistantMsg string // Truncated assistant response
}

// highlightEntry mirrors the minimum fields we need from a .jsonl line
// to extract conversation highlights. We keep this separate from
// scanner's internal types since those are unexported.
type highlightEntry struct {
	Type    string          `json:"type"`    // "user", "assistant", "system", etc.
	IsMeta  bool            `json:"isMeta"`  // True for system-injected messages
	Message *highlightMsg   `json:"message"` // The actual message content
}

// highlightMsg is the nested message payload we need for highlights.
type highlightMsg struct {
	Role    string             `json:"role"`    // "user" or "assistant"
	Content []highlightContent `json:"content"` // Array of content blocks
}

// highlightContent represents a single content block (text, tool_use, etc.).
type highlightContent struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result", etc.
	Text string `json:"text"` // Present when Type == "text"
}

// extractHighlights reads through a .jsonl file and extracts the first N
// user/assistant message pairs. It skips meta/system messages and only
// includes pairs where both sides have meaningful text content.
//
// This re-reads the file (rather than storing messages in Session) to keep
// Session lightweight — most views don't need full message text.
func extractHighlights(filePath string, maxPairs int) []highlight {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	// Allow large lines (some messages contain base64 images)
	const maxLineSize = 10 * 1024 * 1024
	sc.Buffer(make([]byte, 0, maxLineSize), maxLineSize)

	var highlights []highlight

	// We collect messages in order and pair them up: user → assistant.
	// pendingUser holds the last user message text while we wait for
	// the assistant's response.
	var pendingUser string

	for sc.Scan() {
		if len(highlights) >= maxPairs {
			break
		}

		var entry highlightEntry
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil {
			continue
		}

		// Skip meta/system messages — they're injected by Claude Code
		// and don't represent actual conversation.
		if entry.IsMeta {
			continue
		}

		if entry.Message == nil {
			continue
		}

		// Extract the first meaningful text from the message's content blocks.
		text := extractText(entry.Message.Content)
		if text == "" {
			continue
		}

		switch entry.Type {
		case "user":
			// Store the user message; we'll pair it with the next assistant message.
			pendingUser = truncateMessage(text, maxMessageLen)

		case "assistant":
			// Only create a highlight if we have a matching user message.
			if pendingUser != "" {
				highlights = append(highlights, highlight{
					userMsg:      pendingUser,
					assistantMsg: truncateMessage(text, maxMessageLen),
				})
				pendingUser = "" // Reset for the next pair
			}
		}
	}

	return highlights
}

// extractText pulls the first meaningful text from an array of content blocks.
// It skips tool_use, tool_result, and image blocks — we only want human-readable text.
func extractText(blocks []highlightContent) string {
	for _, block := range blocks {
		if block.Type != "text" {
			continue
		}
		text := strings.TrimSpace(block.Text)
		if len(text) < 5 {
			continue
		}
		// Skip image references and system annotations
		if strings.HasPrefix(text, "[Image") || strings.HasPrefix(text, "[Request") {
			continue
		}
		return collapseWhitespace(text)
	}
	return ""
}

// =====================================================================
// HELPER FUNCTIONS
// =====================================================================

// truncateMessage cuts a message to maxLen characters and appends "..."
// if it was truncated. It tries to break at a word boundary.
func truncateMessage(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	// Try to break at a word boundary (last space before maxLen).
	cutoff := strings.LastIndex(text[:maxLen], " ")
	if cutoff > maxLen/2 {
		return text[:cutoff] + "..."
	}
	return text[:maxLen] + "..."
}

// shortenProjectPath takes a full path like "/Users/<user>/git/my-project"
// and returns the last 2 segments joined with " > ": "git > my-project".
// This matches the display format used elsewhere in claix.
func shortenProjectPath(path string) string {
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], " > ")
}

// formatDate extracts "YYYY-MM-DD HH:MM" from an ISO 8601 timestamp.
// Input:  "2026-04-07T07:52:30.123Z"
// Output: "2026-04-07 07:52"
func formatDate(timestamp string) string {
	if len(timestamp) >= 16 {
		// Replace the 'T' separator with a space for readability
		return strings.Replace(timestamp[:16], "T", " ", 1)
	}
	return timestamp
}

// collapseWhitespace replaces all runs of whitespace (spaces, newlines, tabs)
// with a single space. This makes multi-line messages display cleanly on one line.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
