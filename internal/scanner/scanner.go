// Package scanner reads Claude Code session data from the user's filesystem.
//
// Claude Code stores conversation logs as .jsonl files under:
//   ~/.claude/projects/<project-dir>/<session-id>.jsonl
//
// Each line in the .jsonl file is a JSON object representing one event
// in the conversation (user message, assistant response, tool progress, etc).
//
// This package ONLY reads Claude's files — it never writes to or modifies them.
package scanner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// =====================================================================
// SESSION STATUS
// =====================================================================

// Status represents the state of a session — helps users quickly see
// which sessions are worth resuming vs which are done or empty.
type Status string

const (
	StatusActive Status = "active" // Session appears to have unfinished work
	StatusDone   Status = "done"   // Session completed naturally
	StatusEmpty  Status = "empty"  // Session was opened but nothing happened
)

// =====================================================================
// PR LINK
// =====================================================================

// PRLink represents a pull request created during a session.
// Claude Code logs these as "pr-link" entries in the .jsonl file.
type PRLink struct {
	Number     int    // PR number (e.g., 163)
	Repository string // Repo name (e.g., "<user>/my-project")
	URL        string // Full GitHub URL
}

// =====================================================================
// FILE ACTIVITY
// =====================================================================

// FileActivity tracks which files were read or edited during a session.
// We extract this from tool_use blocks in assistant messages.
type FileActivity struct {
	FilesRead   []string // Full paths of files that were read
	FilesEdited []string // Full paths of files that were edited

	// RepoSummary is a human-readable summary like "my-project (12 files), another-project (1 file)"
	// It groups file activity by the top-level repo/folder they belong to.
	RepoSummary string
}

// =====================================================================
// SESSION (the main struct — now much richer)
// =====================================================================

// Session represents a single Claude Code conversation with all its metadata.
type Session struct {
	ID          string // UUID of the session (e.g., "09662c0c-0f57-4a34-8cee-315e5320ee33")
	ProjectDir  string // The raw directory name (e.g., "-Users-<user>-git-my-project")
	ProjectPath string // Human-readable project path (e.g., "/Users/<user>/git/my-project")
	GitBranch   string // Git branch that was active during the session
	StartedAt   string // ISO timestamp of the first event
	LastActive  string // ISO timestamp of the last event
	Preview     string // First meaningful user message (truncated to ~80 chars for list view)
	Description string // Longer version of the first user message (up to 500 chars for detail panel)
	Title       string // Auto-generated title describing what the session was about
	Status      Status // active, done, or empty
	Slug        string // Claude's fun session name (e.g., "warm-prancing-tarjan")
	UserMsgs    int    // Number of user messages
	AssistantMsgs int  // Number of assistant messages
	TotalMsgs   int    // UserMsgs + AssistantMsgs
	Version     string // Claude Code version used
	FilePath    string // Full path to the .jsonl file
	PRLinks     []PRLink     // Pull requests created during this session
	Activity    FileActivity // Files read/edited during this session

	// lastMsgType tracks who sent the last message — used to determine status.
	// Lowercase = unexported (private) — only accessible within this package.
	lastMsgType string
}

// =====================================================================
// JSONL ENTRY TYPES (what each line in the file looks like)
// =====================================================================

// jsonlEntry represents a single line in the .jsonl file.
// We decode all fields we might need — Go silently ignores extra JSON keys.
type jsonlEntry struct {
	Type      string `json:"type"`      // "user", "assistant", "queue-operation", "progress", "system", "pr-link"
	Timestamp string `json:"timestamp"` // ISO 8601 timestamp
	SessionID string `json:"sessionId"` // UUID matching the filename
	GitBranch string `json:"gitBranch"` // Git branch (on user/assistant entries)
	Version   string `json:"version"`   // Claude Code version
	IsMeta    bool   `json:"isMeta"`    // True for system-injected messages
	Slug      string `json:"slug"`      // Fun session name like "warm-prancing-tarjan"

	// Message content — present on "user" and "assistant" type entries
	Message *messagePayload `json:"message"`

	// PR link fields — only present on "pr-link" type entries
	PRNumber     int    `json:"prNumber"`
	PRRepository string `json:"prRepository"`
	PRURL        string `json:"prUrl"`
}

// messagePayload is the nested "message" object inside a jsonlEntry.
type messagePayload struct {
	Role    string           `json:"role"`    // "user" or "assistant"
	Content []contentBlock   `json:"content"` // Array of typed content blocks
}

// contentBlock is one piece of content inside a message.
// It can be text, an image, a tool_use call, or a tool_result.
// We use the same struct for all types — irrelevant fields are just empty.
type contentBlock struct {
	Type  string `json:"type"`  // "text", "image", "tool_use", "tool_result"
	Text  string `json:"text"`  // Present when Type == "text"
	Name  string `json:"name"`  // Tool name when Type == "tool_use" (e.g., "Read", "Edit", "Bash")

	// Input is the arguments passed to a tool call.
	// We use json.RawMessage (raw bytes) because each tool has different input shapes.
	// We'll parse it selectively when we need the file_path field.
	Input json.RawMessage `json:"input"`
}

// toolInput is used to extract just the file_path from a tool call's input.
// For example, a Read tool call has: {"file_path": "/Users/<user>/git/my-project/src/App.tsx"}
type toolInput struct {
	FilePath string `json:"file_path"` // Present in Read, Edit, Write tool calls
}

// =====================================================================
// SCAN ALL SESSIONS
// =====================================================================

// ScanAll finds and parses all Claude Code sessions across all projects.
// Returns sessions sorted by LastActive (most recent first).
// Pass "" for claudeHome to use the default ~/.claude.
func ScanAll(claudeHome string) ([]Session, error) {
	if claudeHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		claudeHome = filepath.Join(homeDir, ".claude")
	}

	projectsDir := filepath.Join(claudeHome, "projects")

	projectEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	var sessions []Session

	for _, projectEntry := range projectEntries {
		if !projectEntry.IsDir() {
			continue
		}

		projectDirName := projectEntry.Name()
		projectPath := filepath.Join(projectsDir, projectDirName)
		humanPath := dirNameToPath(projectDirName)

		files, err := os.ReadDir(projectPath)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			sessionID := strings.TrimSuffix(file.Name(), ".jsonl")
			jsonlPath := filepath.Join(projectPath, file.Name())

			session, err := parseSession(jsonlPath, sessionID, projectDirName, humanPath)
			if err != nil {
				continue
			}

			sessions = append(sessions, session)
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActive > sessions[j].LastActive
	})

	return sessions, nil
}

// =====================================================================
// PARSE A SINGLE SESSION
// =====================================================================

// parseSession reads a single .jsonl file and extracts all session metadata.
// This is the workhorse function — it streams through the file line by line
// and collects: timestamps, messages, tool calls, PR links, and file activity.
func parseSession(filePath, sessionID, projectDir, humanPath string) (Session, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Session{}, err
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	const maxLineSize = 10 * 1024 * 1024 // 10 MB buffer for large lines (images)
	sc.Buffer(make([]byte, 0, maxLineSize), maxLineSize)

	// Collectors — we fill these as we scan through the file
	var (
		firstTimestamp string
		lastTimestamp  string
		gitBranch      string
		version        string
		slug           string
		preview        string
		description    string // Longer version for detail panel
		userMsgs       int
		assistantMsgs  int
		lastMsgType    string // "user" or "assistant" — who spoke last?

		// PR links collected from "pr-link" entries
		prLinks []PRLink

		// File paths collected from tool_use blocks in assistant messages.
		// Using maps as sets — map[string]bool where the key is the file path.
		// In Go, maps are like JS objects: map[string]bool is { [path: string]: boolean }
		filesRead   = make(map[string]bool)
		filesEdited = make(map[string]bool)

		// Title generation: we collect candidate titles from different sources.
		// Priority order: PR title > commit message > first assistant action > preview
		titleFromAssistant string
	)

	for sc.Scan() {
		line := sc.Bytes()

		var entry jsonlEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// ── Track timestamps ──
		if entry.Timestamp != "" {
			if firstTimestamp == "" {
				firstTimestamp = entry.Timestamp
			}
			lastTimestamp = entry.Timestamp
		}

		// ── Grab the session slug (fun name) ──
		if slug == "" && entry.Slug != "" {
			slug = entry.Slug
		}

		// ── Handle each message type ──
		switch entry.Type {

		case "user":
			userMsgs++
			lastMsgType = "user"

			if gitBranch == "" && entry.GitBranch != "" {
				gitBranch = entry.GitBranch
			}
			if version == "" && entry.Version != "" {
				version = entry.Version
			}
			if preview == "" && !entry.IsMeta && entry.Message != nil {
				preview = extractPreview(entry.Message)
				description = extractDescription(entry.Message)
			}

		case "assistant":
			assistantMsgs++
			lastMsgType = "assistant"

			// Extract file paths from tool_use blocks and a title candidate
			if entry.Message != nil {
				for _, block := range entry.Message.Content {
					// ── Extract file paths from tool calls ──
					if block.Type == "tool_use" && len(block.Input) > 0 {
						var inp toolInput
						// json.Unmarshal on block.Input extracts just the file_path field.
						// If the tool doesn't have file_path, inp.FilePath stays empty — that's fine.
						if err := json.Unmarshal(block.Input, &inp); err == nil && inp.FilePath != "" {
							switch block.Name {
							case "Read":
								filesRead[inp.FilePath] = true
							case "Edit", "Write":
								filesEdited[inp.FilePath] = true
							}
						}
					}

					// ── Extract title from assistant's first substantive text ──
					// Look for patterns like "I'll fix...", "Let me implement...",
					// or just take the first sentence of the first real response.
					if block.Type == "text" && titleFromAssistant == "" {
						candidate := extractTitleFromText(block.Text)
						if candidate != "" {
							titleFromAssistant = candidate
						}
					}
				}
			}

		case "pr-link":
			// Claude Code logs PR creation events with repo, number, and URL
			if entry.PRNumber > 0 {
				prLinks = append(prLinks, PRLink{
					Number:     entry.PRNumber,
					Repository: entry.PRRepository,
					URL:        entry.PRURL,
				})
			}
		}
	}

	// ── Determine session status ──
	status := determineStatus(userMsgs, assistantMsgs, lastMsgType)

	// ── Generate the auto title ──
	title := generateTitle(prLinks, titleFromAssistant, preview, gitBranch)

	// ── Build file activity summary ──
	// Convert map keys to slices (Go maps don't have .keys() — you loop over them)
	readPaths := mapKeys(filesRead)
	editPaths := mapKeys(filesEdited)

	activity := FileActivity{
		FilesRead:   readPaths,
		FilesEdited: editPaths,
		RepoSummary: buildRepoSummary(readPaths, editPaths),
	}

	return Session{
		ID:            sessionID,
		ProjectDir:    projectDir,
		ProjectPath:   humanPath,
		GitBranch:     gitBranch,
		StartedAt:     firstTimestamp,
		LastActive:    lastTimestamp,
		Preview:       preview,
		Description:   description,
		Title:         title,
		Status:        status,
		Slug:          slug,
		UserMsgs:      userMsgs,
		AssistantMsgs: assistantMsgs,
		TotalMsgs:     userMsgs + assistantMsgs,
		Version:       version,
		FilePath:      filePath,
		PRLinks:       prLinks,
		Activity:      activity,
		lastMsgType:   lastMsgType,
	}, nil
}

// =====================================================================
// STATUS DETECTION
// =====================================================================

// determineStatus guesses whether a session is active, done, or empty.
//
// Heuristics:
//   - 0 user messages + 0 assistant messages → empty (opened and closed)
//   - Last message was from user → active (you asked something, waiting for/need response)
//   - Last message was from assistant → done (Claude finished responding)
func determineStatus(userMsgs, assistantMsgs int, lastMsgType string) Status {
	if userMsgs == 0 && assistantMsgs == 0 {
		return StatusEmpty
	}
	if lastMsgType == "user" {
		return StatusActive
	}
	return StatusDone
}

// =====================================================================
// AUTO TITLE GENERATION
// =====================================================================

// generateTitle creates a human-readable title for the session.
// It tries multiple sources in priority order:
//   1. PR title (if a PR was created, that's the best summary)
//   2. First meaningful assistant text (often describes what they'll do)
//   3. Preview of the first user message
//   4. Git branch name as fallback
func generateTitle(prLinks []PRLink, assistantTitle, preview, gitBranch string) string {
	// If we have a PR, use just "PR #N" as the title.
	// Don't include "org/repo" anywhere as bare text — terminals interpret
	// it as a URL (e.g., <user>/my-project → http://<user>/my-project).
	if len(prLinks) > 0 {
		pr := prLinks[0]
		// Extract just the repo name (after the slash) to avoid the URL-like format
		repoName := pr.Repository
		if idx := strings.LastIndex(repoName, "/"); idx >= 0 {
			repoName = repoName[idx+1:]
		}
		return fmt.Sprintf("PR #%d — %s", pr.Number, repoName)
	}

	// If we extracted a title from the assistant's response
	if assistantTitle != "" {
		return assistantTitle
	}

	// Fall back to the user's first message as a title
	if preview != "" {
		// Use the preview but cap it shorter for title use
		if len(preview) > 60 {
			return preview[:60] + "..."
		}
		return preview
	}

	// Last resort: use the branch name
	if gitBranch != "" && gitBranch != "HEAD" && gitBranch != "main" && gitBranch != "master" {
		return "Branch: " + gitBranch
	}

	return "(untitled session)"
}

// extractTitleFromText tries to extract a concise title from the assistant's response.
// It looks for common patterns like:
//   - "I'll fix the notification iframe..."
//   - "Let me implement the file upload..."
//   - "Here's a summary of the changes..."
//
// If no pattern matches, it takes the first sentence (up to the first period).
func extractTitleFromText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) < 10 {
		return ""
	}

	// Collapse to single line for easier parsing
	text = collapseWhitespace(text)

	// Patterns that indicate the assistant describing what they're about to do.
	// These are Go string prefixes we check — like text.startsWith() in JS.
	actionPrefixes := []string{
		"I'll ", "I will ", "I've ", "I have ",
		"Let me ", "Let's ",
		"I'm going to ", "I am going to ",
		"I'll now ", "Now I'll ",
		"I can see ", "I notice ",
		"The issue is ", "The problem is ",
		"This fixes ", "This implements ", "This adds ",
	}

	for _, prefix := range actionPrefixes {
		// strings.HasPrefix is case-sensitive, so we also check lowercase
		if strings.HasPrefix(text, prefix) || strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix)) {
			// Take text up to the first sentence boundary
			sentence := extractFirstSentence(text)
			if len(sentence) > 70 {
				return sentence[:70] + "..."
			}
			return sentence
		}
	}

	// No pattern matched — try taking the first sentence if it's short enough
	first := extractFirstSentence(text)
	if len(first) > 10 && len(first) <= 80 {
		return first
	}

	return ""
}

// extractFirstSentence returns the text up to the first sentence-ending punctuation.
func extractFirstSentence(text string) string {
	// Look for sentence boundaries: . ! ? followed by space or end of string
	for i, ch := range text {
		if (ch == '.' || ch == '!' || ch == '?') && (i+1 >= len(text) || text[i+1] == ' ') {
			return strings.TrimSpace(text[:i+1])
		}
	}

	// No sentence boundary found — return up to 80 chars
	if len(text) > 80 {
		// Try to break at a word boundary
		cutoff := strings.LastIndex(text[:80], " ")
		if cutoff > 40 {
			return text[:cutoff] + "..."
		}
		return text[:80] + "..."
	}
	return text
}

// =====================================================================
// FILE ACTIVITY HELPERS
// =====================================================================

// buildRepoSummary groups file paths by their top-level repo/folder
// and returns a human-readable summary.
//
// Example input paths:
//   /Users/<user>/git/my-project/src/App.tsx
//   /Users/<user>/git/my-project/src/components/Foo.tsx
//   /Users/<user>/git/another-project/scripts/plan.sh
//
// Output: "my-project (2 files), another-project (1 file)"
func buildRepoSummary(readPaths, editPaths []string) string {
	// Combine all paths and count files per repo
	// repoCounts maps repo name → number of unique files
	repoCounts := make(map[string]int)

	// Use a set to avoid double-counting files that were both read and edited
	allPaths := make(map[string]bool)
	for _, p := range readPaths {
		allPaths[p] = true
	}
	for _, p := range editPaths {
		allPaths[p] = true
	}

	for path := range allPaths {
		repo := extractRepoName(path)
		if repo != "" {
			repoCounts[repo]++
		}
	}

	if len(repoCounts) == 0 {
		return ""
	}

	// Sort repos by file count (highest first) for consistent display.
	// In Go, you can't sort a map directly — you sort the keys and iterate.
	type repoCount struct {
		name  string
		count int
	}
	var sorted []repoCount
	for name, count := range repoCounts {
		sorted = append(sorted, repoCount{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// Build the summary string
	var parts []string
	for _, rc := range sorted {
		word := "files"
		if rc.count == 1 {
			word = "file"
		}
		parts = append(parts, fmt.Sprintf("%s (%d %s)", rc.name, rc.count, word))
	}

	return strings.Join(parts, ", ")
}

// extractRepoName pulls the repo/project folder name from a full file path.
// It looks for common base directories (git/, Documents/, etc.) and takes
// the next segment as the repo name.
//
// "/Users/<user>/git/my-project/src/App.tsx" → "my-project"
// "/Users/<user>/Documents/personal/claix/main.go" → "claix"
// "/Users/<user>/.zshrc" → ".zshrc" (config file, no repo)
func extractRepoName(path string) string {
	parts := strings.Split(path, string(os.PathSeparator))

	// Look for common project base directories
	baseDirs := []string{"git", "Documents", "projects", "repos", "src", "code", "dev", "work"}

	for i, part := range parts {
		for _, base := range baseDirs {
			if part == base && i+1 < len(parts) {
				// The next segment after the base dir is the repo name
				return parts[i+1]
			}
		}
	}

	// Fallback: if path is in the home directory, use the deepest meaningful folder
	// e.g., /Users/<user>/.zshrc → use the filename
	if len(parts) >= 2 {
		return parts[len(parts)-2] // Parent folder of the file
	}

	return ""
}

// mapKeys extracts the keys from a map[string]bool into a string slice.
// Go doesn't have Object.keys() — you have to build it manually.
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m)) // make([]string, 0, N) pre-allocates capacity N
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Sort for consistent output
	return keys
}

// =====================================================================
// PREVIEW EXTRACTION
// =====================================================================

// extractPreview pulls the first meaningful text from a message's content blocks.
// Returns a single-line, truncated string (max 80 chars) suitable for display.
func extractPreview(msg *messagePayload) string {
	for _, block := range msg.Content {
		if block.Type != "text" {
			continue
		}

		text := strings.TrimSpace(block.Text)
		if len(text) < 10 {
			continue
		}

		if strings.HasPrefix(text, "[Image") || strings.HasPrefix(text, "[Request") {
			continue
		}

		text = collapseWhitespace(text)

		// Remove embedded image references
		for {
			start := strings.Index(text, "[Image")
			if start == -1 {
				break
			}
			end := strings.Index(text[start:], "]")
			if end == -1 {
				break
			}
			text = text[:start] + text[start+end+1:]
		}
		text = collapseWhitespace(strings.TrimSpace(text))

		if len(text) < 10 {
			continue
		}

		if len(text) > 80 {
			return text[:80] + "..."
		}
		return text
	}

	return ""
}

// extractDescription is like extractPreview but returns a longer version (up to 500 chars)
// for the detail panel. It preserves more of the original message.
func extractDescription(msg *messagePayload) string {
	for _, block := range msg.Content {
		if block.Type != "text" {
			continue
		}

		text := strings.TrimSpace(block.Text)
		if len(text) < 10 {
			continue
		}

		if strings.HasPrefix(text, "[Image") || strings.HasPrefix(text, "[Request") {
			continue
		}

		text = collapseWhitespace(text)

		// Remove embedded image references
		for {
			start := strings.Index(text, "[Image")
			if start == -1 {
				break
			}
			end := strings.Index(text[start:], "]")
			if end == -1 {
				break
			}
			text = text[:start] + text[start+end+1:]
		}
		text = collapseWhitespace(strings.TrimSpace(text))

		if len(text) < 10 {
			continue
		}

		if len(text) > 500 {
			return text[:500] + "..."
		}
		return text
	}

	return ""
}

// collapseWhitespace replaces all runs of whitespace with a single space.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// =====================================================================
// PATH RESOLUTION (Claude's directory naming → real filesystem path)
// =====================================================================

// dirNameToPath converts Claude's dash-separated directory name back to a real path.
// "-Users-<user>-git-my-project" → "/Users/<user>/git/my-project"
func dirNameToPath(dirName string) string {
	if len(dirName) == 0 {
		return dirName
	}

	path := "/" + strings.ReplaceAll(dirName[1:], "-", "/")

	if _, err := os.Stat(path); err == nil {
		return path
	}

	return resolveRealPath(dirName)
}

// resolveRealPath tries to reconstruct the real filesystem path by checking
// which combinations of dash-separated segments actually exist on disk.
func resolveRealPath(dirName string) string {
	if len(dirName) == 0 {
		return dirName
	}

	parts := strings.Split(dirName[1:], "-")
	current := "/"
	i := 0

	for i < len(parts) {
		found := false
		for j := len(parts); j > i; j-- {
			candidate := strings.Join(parts[i:j], "-")
			candidatePath := filepath.Join(current, candidate)

			if _, err := os.Stat(candidatePath); err == nil {
				current = candidatePath
				i = j
				found = true
				break
			}
		}

		if !found {
			current = filepath.Join(current, parts[i])
			i++
		}
	}

	return current
}
