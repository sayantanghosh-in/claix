// Package mcp implements a Model Context Protocol (MCP) server for claix.
//
// MCP lets Claude Code talk to external tools mid-session using JSON-RPC 2.0
// over stdio. When someone configures claix as an MCP server in their Claude
// Code settings, Claude can call tools like "tag this session" or "list sessions"
// without the user leaving their conversation.
//
// Protocol flow:
//   1. Claude Code launches `claix mcp-server` as a subprocess
//   2. It sends JSON-RPC requests on stdin (one JSON object per line)
//   3. This server processes each request and writes JSON responses to stdout
//   4. All logging goes to stderr (stdout is strictly for JSON-RPC)
//
// Reference: https://modelcontextprotocol.io/
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sayantanghosh-in/claix/internal/scanner"
)

// =====================================================================
// JSON-RPC 2.0 TYPES
// =====================================================================
// These structs match the JSON-RPC 2.0 spec that MCP uses.
// Every message has "jsonrpc": "2.0" and an "id" to match requests/responses.

// JSONRPCRequest is what Claude Code sends us. The "method" field tells us
// what it wants (e.g., "initialize", "tools/list", "tools/call").
// Params holds the method-specific arguments as raw JSON.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is what we send back. Either Result or Error is set, not both.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error with a numeric code and message.
// Standard codes: -32600 (invalid request), -32601 (method not found),
// -32602 (invalid params), -32603 (internal error).
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// =====================================================================
// MCP-SPECIFIC TYPES
// =====================================================================

// ToolDefinition describes a tool that Claude can call. The InputSchema
// is a JSON Schema object that tells Claude what parameters to send.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolCallParams is what we receive when Claude calls "tools/call".
// Name identifies which tool, Arguments holds the tool-specific input.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult is the MCP format for returning tool output to Claude.
// Content is an array of content blocks (we use text blocks).
type ToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent is a single content block in a tool result.
// Type is always "text" for our tools.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// =====================================================================
// STORE TYPES — persisted to ~/.config/claix/store.json
// =====================================================================
// The store file holds user-added metadata (tags, notes) for sessions.
// This is separate from Claude's own session files — we never modify those.
// When the store package is fully implemented, it will use this same format.

// StoreData is the top-level structure of the store.json file.
type StoreData struct {
	// Sessions maps session ID → metadata. Using a map means O(1) lookups.
	Sessions map[string]*SessionMeta `json:"sessions"`
}

// SessionMeta holds user-added metadata for a single session.
type SessionMeta struct {
	Tags      []string `json:"tags,omitempty"`      // User-defined labels like "auth-refactor"
	Notes     []Note   `json:"notes,omitempty"`      // Timestamped notes about the session
	Pinned    bool     `json:"pinned,omitempty"`     // Whether the session is pinned to top
	UpdatedAt string   `json:"updated_at,omitempty"` // ISO timestamp of last metadata change
}

// Note is a timestamped text note attached to a session.
type Note struct {
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// =====================================================================
// STORE OPERATIONS
// =====================================================================
// These functions read/write the store.json file with a mutex to prevent
// concurrent writes from corrupting the file.

// storeMu protects concurrent access to the store file.
// Even though our server is single-threaded (processing one request at a time),
// this is good practice and future-proofs against concurrency.
var storeMu sync.Mutex

// storePath returns the full path to the store.json file.
// It creates the config directory if it doesn't exist.
func storePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	dir := filepath.Join(homeDir, ".config", "claix")

	// os.MkdirAll is like `mkdir -p` — creates all parent dirs if needed.
	// 0755 means: owner can read/write/execute, others can read/execute.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create config directory: %w", err)
	}
	return filepath.Join(dir, "store.json"), nil
}

// loadStore reads and parses the store.json file.
// If the file doesn't exist yet, it returns an empty StoreData (not an error).
func loadStore() (*StoreData, error) {
	path, err := storePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet — return empty store.
			// This is the normal case on first use.
			return &StoreData{Sessions: make(map[string]*SessionMeta)}, nil
		}
		return nil, fmt.Errorf("cannot read store: %w", err)
	}

	var store StoreData
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("corrupt store.json: %w", err)
	}

	// Ensure the map is initialized even if the file had "sessions": null
	if store.Sessions == nil {
		store.Sessions = make(map[string]*SessionMeta)
	}

	return &store, nil
}

// saveStore writes the StoreData to disk as formatted JSON.
// It uses json.MarshalIndent for human readability (users may want to inspect it).
func saveStore(store *StoreData) error {
	path, err := storePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal store: %w", err)
	}

	// 0644 = owner can read/write, others can read only.
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write store: %w", err)
	}

	return nil
}

// getOrCreateSession returns the SessionMeta for a given ID, creating it if needed.
func getOrCreateSession(store *StoreData, sessionID string) *SessionMeta {
	meta, exists := store.Sessions[sessionID]
	if !exists {
		meta = &SessionMeta{}
		store.Sessions[sessionID] = meta
	}
	return meta
}

// =====================================================================
// TOOL DEFINITIONS
// =====================================================================
// Each tool has a JSON Schema describing its inputs. Claude uses these
// schemas to know what parameters to send when calling the tool.

// tools returns the list of all tools this MCP server exposes.
func tools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "claix_tag_session",
			Description: "Add a tag/label to a Claude Code session. Tags help organize and find sessions later (e.g., 'auth-refactor', 'bugfix', 'experiment').",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session UUID to tag. You can find this in the session list or from the current session context.",
					},
					"tag": map[string]interface{}{
						"type":        "string",
						"description": "The tag to add (e.g., 'auth-refactor', 'wip', 'ready-for-review').",
					},
				},
				"required": []string{"session_id", "tag"},
			},
		},
		{
			Name:        "claix_note_session",
			Description: "Add a timestamped note to a Claude Code session. Notes help you remember context, like where you left off or what to do next.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session UUID to annotate.",
					},
					"note": map[string]interface{}{
						"type":        "string",
						"description": "The note text (e.g., 'Left off at migration step 3', 'Needs review before merging').",
					},
				},
				"required": []string{"session_id", "note"},
			},
		},
		{
			Name:        "claix_list_sessions",
			Description: "List recent Claude Code sessions across all projects. Returns session IDs, titles, status, and project info.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of sessions to return. Defaults to 10.",
					},
				},
			},
		},
		{
			Name:        "claix_session_info",
			Description: "Get detailed information about a specific Claude Code session, including tags, notes, file activity, and PR links.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "The session UUID to look up.",
					},
				},
				"required": []string{"session_id"},
			},
		},
	}
}

// =====================================================================
// TOOL HANDLERS
// =====================================================================
// Each handler function processes a specific tool call and returns a result.

// handleTagSession adds a tag to a session's metadata in the store.
func handleTagSession(args json.RawMessage) (*ToolResult, error) {
	// Parse the arguments Claude sent us
	var params struct {
		SessionID string `json:"session_id"`
		Tag       string `json:"tag"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if params.SessionID == "" || params.Tag == "" {
		return errorResult("session_id and tag are required"), nil
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	meta := getOrCreateSession(store, params.SessionID)

	// Check if the tag already exists to avoid duplicates.
	// Go doesn't have Array.includes() — we loop through manually.
	for _, existing := range meta.Tags {
		if existing == params.Tag {
			return textResult(fmt.Sprintf("Session %s already has tag '%s'.", shortID(params.SessionID), params.Tag)), nil
		}
	}

	meta.Tags = append(meta.Tags, params.Tag)
	meta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := saveStore(store); err != nil {
		return nil, err
	}

	return textResult(fmt.Sprintf("Tagged session %s with '%s'. Tags: %v", shortID(params.SessionID), params.Tag, meta.Tags)), nil
}

// handleNoteSession adds a timestamped note to a session.
func handleNoteSession(args json.RawMessage) (*ToolResult, error) {
	var params struct {
		SessionID string `json:"session_id"`
		NoteText  string `json:"note"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if params.SessionID == "" || params.NoteText == "" {
		return errorResult("session_id and note are required"), nil
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	meta := getOrCreateSession(store, params.SessionID)
	meta.Notes = append(meta.Notes, Note{
		Text:      params.NoteText,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	meta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := saveStore(store); err != nil {
		return nil, err
	}

	return textResult(fmt.Sprintf("Added note to session %s: \"%s\"", shortID(params.SessionID), params.NoteText)), nil
}

// handleListSessions scans all Claude Code sessions and returns a summary.
func handleListSessions(args json.RawMessage) (*ToolResult, error) {
	// Parse optional limit parameter
	var params struct {
		Limit int `json:"limit"`
	}
	// Ignore unmarshal errors — limit defaults to 0 which we treat as 10
	_ = json.Unmarshal(args, &params)

	if params.Limit <= 0 {
		params.Limit = 10
	}

	// Use the scanner package to find all sessions
	sessions, err := scanner.ScanAll("")
	if err != nil {
		return nil, fmt.Errorf("failed to scan sessions: %w", err)
	}

	// Load the store to merge in tags/notes metadata
	store, err := loadStore()
	if err != nil {
		// Non-fatal — we can still show sessions without metadata
		log.Printf("warning: could not load store: %v", err)
		store = &StoreData{Sessions: make(map[string]*SessionMeta)}
	}

	// Cap to the requested limit
	if len(sessions) > params.Limit {
		sessions = sessions[:params.Limit]
	}

	// Build the response as a JSON array of session summaries.
	// We create a simpler struct for the output to avoid leaking internal fields.
	type sessionSummary struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Status      string   `json:"status"`
		ProjectPath string   `json:"project_path"`
		GitBranch   string   `json:"git_branch,omitempty"`
		LastActive  string   `json:"last_active"`
		Tags        []string `json:"tags,omitempty"`
		NoteCount   int      `json:"note_count,omitempty"`
	}

	summaries := make([]sessionSummary, 0, len(sessions))
	for _, s := range sessions {
		// Skip empty sessions — they have no useful content
		if s.Status == scanner.StatusEmpty {
			continue
		}

		summary := sessionSummary{
			ID:          s.ID,
			Title:       s.Title,
			Status:      string(s.Status),
			ProjectPath: s.ProjectPath,
			GitBranch:   s.GitBranch,
			LastActive:  s.LastActive,
		}

		// Merge in store metadata if available
		if meta, ok := store.Sessions[s.ID]; ok {
			summary.Tags = meta.Tags
			summary.NoteCount = len(meta.Notes)
		}

		summaries = append(summaries, summary)
	}

	// Marshal the summaries as pretty JSON for Claude to read
	output, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sessions: %w", err)
	}

	return textResult(string(output)), nil
}

// handleSessionInfo returns detailed information about a single session.
func handleSessionInfo(args json.RawMessage) (*ToolResult, error) {
	var params struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if params.SessionID == "" {
		return errorResult("session_id is required"), nil
	}

	// Scan all sessions and find the one matching the requested ID
	sessions, err := scanner.ScanAll("")
	if err != nil {
		return nil, fmt.Errorf("failed to scan sessions: %w", err)
	}

	var found *scanner.Session
	for i := range sessions {
		if sessions[i].ID == params.SessionID {
			found = &sessions[i]
			break
		}
	}

	if found == nil {
		return errorResult(fmt.Sprintf("Session '%s' not found.", params.SessionID)), nil
	}

	// Load store metadata
	store, err := loadStore()
	if err != nil {
		log.Printf("warning: could not load store: %v", err)
		store = &StoreData{Sessions: make(map[string]*SessionMeta)}
	}

	// Simplified PR link for output (declared before sessionDetail so it can be used)
	type prLink struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}

	// Build a detailed response struct
	type sessionDetail struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Status        string   `json:"status"`
		ProjectPath   string   `json:"project_path"`
		GitBranch     string   `json:"git_branch,omitempty"`
		StartedAt     string   `json:"started_at"`
		LastActive    string   `json:"last_active"`
		Preview       string   `json:"preview,omitempty"`
		Description   string   `json:"description,omitempty"`
		Slug          string   `json:"slug,omitempty"`
		UserMessages  int      `json:"user_messages"`
		TotalMessages int      `json:"total_messages"`
		Version       string   `json:"version,omitempty"`
		Tags          []string `json:"tags,omitempty"`
		Notes         []Note   `json:"notes,omitempty"`
		FilesEdited   []string `json:"files_edited,omitempty"`
		FilesRead     []string `json:"files_read,omitempty"`
		PRLinks       []prLink `json:"pr_links,omitempty"`
	}

	detail := sessionDetail{
		ID:            found.ID,
		Title:         found.Title,
		Status:        string(found.Status),
		ProjectPath:   found.ProjectPath,
		GitBranch:     found.GitBranch,
		StartedAt:     found.StartedAt,
		LastActive:    found.LastActive,
		Preview:       found.Preview,
		Description:   found.Description,
		Slug:          found.Slug,
		UserMessages:  found.UserMsgs,
		TotalMessages: found.TotalMsgs,
		Version:       found.Version,
		FilesEdited:   found.Activity.FilesEdited,
		FilesRead:     found.Activity.FilesRead,
	}

	// Add PR links
	for _, pr := range found.PRLinks {
		// We need to use an anonymous struct here since prLink type is declared
		// inside the function after the detail struct that references it.
		detail.PRLinks = append(detail.PRLinks, prLink{
			Number: pr.Number,
			URL:    pr.URL,
		})
	}

	// Merge store metadata
	if meta, ok := store.Sessions[found.ID]; ok {
		detail.Tags = meta.Tags
		detail.Notes = meta.Notes
	}

	output, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session info: %w", err)
	}

	return textResult(string(output)), nil
}

// =====================================================================
// HELPER FUNCTIONS
// =====================================================================

// textResult creates a successful tool result with a text content block.
func textResult(text string) *ToolResult {
	return &ToolResult{
		Content: []ToolContent{{Type: "text", Text: text}},
	}
}

// errorResult creates a tool result that indicates an error to Claude.
// This is different from a JSON-RPC error — it means "the tool ran but
// encountered a problem" (like "session not found").
func errorResult(text string) *ToolResult {
	return &ToolResult{
		Content: []ToolContent{{Type: "text", Text: text}},
		IsError: true,
	}
}

// shortID returns the first 8 characters of a session ID for display.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// =====================================================================
// MAIN SERVER LOOP
// =====================================================================

// Serve starts the MCP server. It reads JSON-RPC requests from stdin
// line by line, dispatches them to the appropriate handler, and writes
// JSON-RPC responses to stdout. Logging goes to stderr.
//
// This function blocks forever (until stdin is closed or the process is killed).
// Claude Code manages the lifecycle — it starts us when needed and kills us
// when the session ends.
func Serve() {
	// Direct all log output to stderr. Stdout is exclusively for JSON-RPC.
	// This is critical — any stray output on stdout would corrupt the protocol.
	log.SetOutput(os.Stderr)
	log.SetPrefix("[claix-mcp] ")
	log.Println("MCP server starting...")

	// bufio.Scanner reads stdin line by line. Each line is one JSON-RPC request.
	input := bufio.NewScanner(os.Stdin)

	// Increase the buffer size — some requests (like tool calls with large args)
	// can be several KB. 1 MB should be more than enough.
	const maxInputSize = 1 * 1024 * 1024
	input.Buffer(make([]byte, 0, maxInputSize), maxInputSize)

	// The encoder writes JSON to stdout. We use it instead of fmt.Println
	// to ensure correct JSON encoding.
	encoder := json.NewEncoder(os.Stdout)

	for input.Scan() {
		line := input.Bytes()

		// Skip empty lines (some transports add blank lines between messages)
		if len(line) == 0 {
			continue
		}

		// Parse the incoming JSON-RPC request
		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("failed to parse request: %v", err)
			// Send a parse error response (JSON-RPC spec requires this)
			_ = encoder.Encode(JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &RPCError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		log.Printf("received: method=%s id=%v", req.Method, req.ID)

		// Dispatch based on the method name
		response := handleRequest(req)

		// Write the response as a single JSON line to stdout
		if err := encoder.Encode(response); err != nil {
			log.Printf("failed to write response: %v", err)
		}
	}

	// If we get here, stdin was closed — Claude Code is done with us
	if err := input.Err(); err != nil {
		log.Printf("stdin read error: %v", err)
	}
	log.Println("MCP server shutting down.")
}

// handleRequest dispatches a JSON-RPC request to the appropriate handler.
// It returns a JSONRPCResponse that will be sent back to Claude Code.
func handleRequest(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {

	// ── initialize: Tell Claude Code who we are and what we can do ──
	case "initialize":
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo": map[string]interface{}{
					"name":    "claix",
					"version": "0.1.0",
				},
			},
		}

	// ── notifications/initialized: Claude Code confirms it received our init ──
	// This is a notification (no response needed), but we send one anyway
	// since some implementations expect it.
	case "notifications/initialized":
		log.Println("client confirmed initialization")
		// Notifications don't get responses in JSON-RPC, but MCP expects us
		// to be silent here. Return an empty result if an ID was provided.
		if req.ID != nil {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]interface{}{},
			}
		}
		// No ID means it's a true notification — don't respond.
		// But we need to return something from this function, so return
		// a response that we'll skip encoding. We handle this by checking
		// for nil ID in the caller... actually, let's just return it.
		// The MCP spec says notifications have no ID, so we won't encode this.
		return JSONRPCResponse{JSONRPC: "2.0"}

	// ── tools/list: Return our available tools ──
	case "tools/list":
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": tools(),
			},
		}

	// ── tools/call: Execute a specific tool ──
	case "tools/call":
		return handleToolCall(req)

	// ── Unknown method ──
	default:
		log.Printf("unknown method: %s", req.Method)
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

// handleToolCall parses the tools/call params and dispatches to the right handler.
func handleToolCall(req JSONRPCRequest) JSONRPCResponse {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "Invalid tool call params"},
		}
	}

	log.Printf("tool call: %s", params.Name)

	// Dispatch to the handler for this tool name
	var result *ToolResult
	var err error

	switch params.Name {
	case "claix_tag_session":
		result, err = handleTagSession(params.Arguments)
	case "claix_note_session":
		result, err = handleNoteSession(params.Arguments)
	case "claix_list_sessions":
		result, err = handleListSessions(params.Arguments)
	case "claix_session_info":
		result, err = handleSessionInfo(params.Arguments)
	default:
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Unknown tool: %s", params.Name),
			},
		}
	}

	// If the handler returned an internal error, convert it to a JSON-RPC error
	if err != nil {
		log.Printf("tool error: %v", err)
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32603, Message: err.Error()},
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}
