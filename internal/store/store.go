// Package store manages claix's own metadata — tags, notes, pinned sessions,
// and a cached index of scanned sessions.
//
// This is completely separate from Claude Code's .jsonl files.
// Claude's files are read-only (via the scanner package); the store package
// manages claix's own data at ~/.config/claix/.
//
// Two files are stored:
//   - store.json  — user-created metadata (tags, notes, pins) per session
//   - index.json  — cached session list from the last `claix sync`
//
// Think of this like localStorage in the browser — a simple JSON file
// that persists between runs of the CLI.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	// We import the scanner package to reference the Session type in IndexCache.
	// This creates a dependency: store depends on scanner, but scanner does NOT
	// depend on store. One-way dependencies like this are good — circular imports
	// are not allowed in Go (unlike JS/TS where circular imports sometimes work).
	"github.com/sayantanghosh-in/claix/internal/scanner"
)

// =====================================================================
// TYPES
// =====================================================================

// SessionMeta holds user-created metadata for a single session.
// These are things claix adds on top of what Claude Code provides —
// the user can tag, pin, or annotate sessions to organize them.
//
// The `json:"...,omitempty"` tags tell Go's JSON encoder to skip fields
// that are empty/zero-valued. This keeps the JSON file clean — a session
// with no tags won't have a "tags" key at all.
type SessionMeta struct {
	Tags   []string `json:"tags,omitempty"`   // User-defined labels (e.g., "bugfix", "wip")
	Notes  string   `json:"notes,omitempty"`  // Free-form notes about the session
	Pinned bool     `json:"pinned,omitempty"` // Pinned sessions always appear at the top
	Title  string   `json:"title,omitempty"`  // User-defined title (from claix init)
}

// Store is the top-level structure persisted to store.json.
// It maps session IDs (UUIDs) to their metadata.
//
// In Go, map[string]SessionMeta is like Record<string, SessionMeta> in TypeScript.
type Store struct {
	Sessions       map[string]SessionMeta `json:"sessions"`
	Theme          string                 `json:"theme,omitempty"`           // Active theme name
	FirstOpened    string                 `json:"first_opened,omitempty"`    // ISO timestamp of first claix launch
	LastStarPrompt string                 `json:"last_star_prompt,omitempty"` // ISO timestamp of last star nudge shown
	StarDismissed  bool                   `json:"star_dismissed,omitempty"`  // True = never show star nudge again
	PendingInit    *PendingInit           `json:"pending_init,omitempty"`    // Metadata set by `claix init` before launching claude
}

// PendingInit holds metadata set by `claix init` that gets applied to
// the next session created in the same working directory.
type PendingInit struct {
	Title     string   `json:"title,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	CreatedAt string   `json:"created_at"`
	Cwd       string   `json:"cwd"`
}

// IndexCache holds the results of a `claix sync` — a snapshot of all sessions
// found by the scanner. This lets the TUI load instantly without re-scanning
// every time (scanning reads many .jsonl files and can be slow).
type IndexCache struct {
	Sessions  []scanner.Session `json:"sessions"`   // All sessions from the last scan
	UpdatedAt string            `json:"updated_at"` // ISO timestamp of when the scan happened
}

// =====================================================================
// FILE PATHS
// =====================================================================

// configDir returns the base directory for claix config: ~/.config/claix/
// os.UserConfigDir() returns the platform-appropriate config directory:
//   - macOS: ~/Library/Application Support (but we use ~/.config for consistency)
//   - Linux: ~/.config
//   - Windows: %AppData%
//
// We append "claix" to create our own namespace within the config directory.
//
// Note: We use ~/.config directly instead of os.UserConfigDir() on macOS because
// ~/Library/Application Support is unusual for CLI tools. Most CLI tools on macOS
// use ~/.config (e.g., git, nvim, starship). This matches user expectations.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "claix"), nil
}

// ensureConfigDir creates the config directory if it doesn't exist.
// os.MkdirAll is like `mkdir -p` — it creates parent directories too,
// and does nothing if the directory already exists. The 0o755 is Unix
// file permissions (owner: rwx, group: rx, others: rx).
func ensureConfigDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// storePath returns the full path to store.json.
func storePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "store.json"), nil
}

// indexPath returns the full path to index.json.
func indexPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "index.json"), nil
}

// =====================================================================
// STORE OPERATIONS
// =====================================================================

// Load reads the store from ~/.config/claix/store.json.
// If the file doesn't exist yet, it returns an empty store (not an error).
// This is the Go equivalent of: JSON.parse(fs.readFileSync(...)) with a fallback.
func Load() (*Store, error) {
	path, err := storePath()
	if err != nil {
		return nil, err
	}

	// os.ReadFile reads the entire file into memory as a byte slice ([]byte).
	// In Go, file I/O returns errors instead of throwing exceptions.
	data, err := os.ReadFile(path)
	if err != nil {
		// os.IsNotExist checks if the error is "file not found".
		// This is like catching ENOENT in Node.js.
		if os.IsNotExist(err) {
			// Return an empty store — the file will be created on first Save().
			return &Store{
				Sessions: make(map[string]SessionMeta),
			}, nil
		}
		return nil, err
	}

	// json.Unmarshal parses JSON bytes into a Go struct.
	// The & operator gives us a pointer to the Store — like passing by reference.
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	// If the JSON was valid but had no "sessions" key, initialize the map.
	// In Go, an uninitialized map is nil, and writing to a nil map panics.
	// Always check and initialize maps before using them.
	if s.Sessions == nil {
		s.Sessions = make(map[string]SessionMeta)
	}

	return &s, nil
}

// Save writes the store to ~/.config/claix/store.json.
// It creates the config directory if it doesn't exist.
//
// json.MarshalIndent produces pretty-printed JSON (like JSON.stringify(obj, null, 2)).
// The "" and "  " arguments are the prefix and indent strings.
func (s *Store) Save() error {
	dir, err := ensureConfigDir()
	if err != nil {
		return err
	}

	// json.MarshalIndent converts the struct to formatted JSON bytes.
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "store.json")

	// os.WriteFile writes bytes to a file, creating it if needed.
	// 0o644 = owner can read/write, everyone else can read.
	return os.WriteFile(path, data, 0o644)
}

// =====================================================================
// SESSION METADATA CRUD
// =====================================================================

// ensureSession makes sure the session exists in the map before modifying it.
// In Go, reading a missing map key returns the zero value (empty SessionMeta),
// but you need to explicitly set it back if you want to modify it.
// This helper avoids that boilerplate.
func (s *Store) ensureSession(sessionID string) SessionMeta {
	meta, exists := s.Sessions[sessionID]
	if !exists {
		meta = SessionMeta{}
	}
	return meta
}

// AddTag adds a tag to a session. If the tag already exists, it's a no-op.
// Tags are simple strings like "bugfix", "wip", "important".
func (s *Store) AddTag(sessionID, tag string) {
	meta := s.ensureSession(sessionID)

	// Check if the tag already exists to avoid duplicates.
	// Go doesn't have Array.includes() — you loop through the slice manually.
	for _, existing := range meta.Tags {
		if existing == tag {
			return // Already tagged, nothing to do
		}
	}

	// append() adds an element to a slice — like Array.push() in JS.
	// It returns a new slice (slices in Go are value types that reference
	// an underlying array, so append may allocate a new array if needed).
	meta.Tags = append(meta.Tags, tag)
	s.Sessions[sessionID] = meta
}

// RemoveTag removes a tag from a session. If the tag doesn't exist, it's a no-op.
func (s *Store) RemoveTag(sessionID, tag string) {
	meta, exists := s.Sessions[sessionID]
	if !exists {
		return
	}

	// Filter out the tag — Go doesn't have Array.filter(), so we build a new slice.
	// This pattern is idiomatic Go: create a new slice, append items that pass the filter.
	var filtered []string
	for _, t := range meta.Tags {
		if t != tag {
			filtered = append(filtered, t)
		}
	}
	meta.Tags = filtered
	s.Sessions[sessionID] = meta
}

// SetNotes sets free-form notes for a session, replacing any existing notes.
func (s *Store) SetNotes(sessionID, notes string) {
	meta := s.ensureSession(sessionID)
	meta.Notes = notes
	s.Sessions[sessionID] = meta
}

// TogglePin flips the pinned state of a session.
// Pinned sessions appear at the top of lists in the TUI.
func (s *Store) TogglePin(sessionID string) {
	meta := s.ensureSession(sessionID)
	meta.Pinned = !meta.Pinned
	s.Sessions[sessionID] = meta
}

// GetMeta returns the metadata for a session.
// If the session has no metadata, returns an empty SessionMeta (zero value).
// In Go, accessing a missing map key returns the zero value of the value type,
// which for SessionMeta is {Tags: nil, Notes: "", Pinned: false}.
func (s *Store) GetMeta(sessionID string) SessionMeta {
	return s.Sessions[sessionID]
}

// SetMeta replaces the full metadata for a session.
func (s *Store) SetMeta(sessionID string, meta SessionMeta) {
	s.Sessions[sessionID] = meta
}

// =====================================================================
// STAR NUDGE
// =====================================================================

// ShouldShowStarNudge determines whether to show the "Star us on GitHub" banner.
// Returns true if: more than 10 days since first open AND more than 10 days since
// last prompt AND the user hasn't permanently dismissed it.
func (s *Store) ShouldShowStarNudge() bool {
	if s.StarDismissed {
		return false
	}

	now := time.Now().UTC()

	// First time ever — record it and don't show yet
	if s.FirstOpened == "" {
		s.FirstOpened = now.Format(time.RFC3339)
		_ = s.Save()
		return false
	}

	firstOpened, err := time.Parse(time.RFC3339, s.FirstOpened)
	if err != nil {
		return false
	}

	// Less than 10 days since first open — too early
	if now.Sub(firstOpened) < 10*24*time.Hour {
		return false
	}

	// Check if we prompted recently
	if s.LastStarPrompt != "" {
		lastPrompt, err := time.Parse(time.RFC3339, s.LastStarPrompt)
		if err == nil && now.Sub(lastPrompt) < 10*24*time.Hour {
			return false
		}
	}

	// Record that we're showing the prompt
	s.LastStarPrompt = now.Format(time.RFC3339)
	_ = s.Save()
	return true
}

// DismissStarNudge hides the banner for 10 more days.
func (s *Store) DismissStarNudge() {
	s.LastStarPrompt = time.Now().UTC().Format(time.RFC3339)
}

// MarkStarred permanently hides the star banner (trusts the user).
func (s *Store) MarkStarred() {
	s.StarDismissed = true
}

// =====================================================================
// PENDING INIT (from `claix init`)
// =====================================================================

// SetPendingInit saves metadata that will be applied to the next session
// started in the given working directory.
func (s *Store) SetPendingInit(title string, tags []string, cwd string) {
	s.PendingInit = &PendingInit{
		Title:     title,
		Tags:      tags,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Cwd:       cwd,
	}
}

// ConsumePendingInit returns and clears the pending init if it matches
// the given working directory. Returns nil if no match.
func (s *Store) ConsumePendingInit(cwd string) *PendingInit {
	if s.PendingInit == nil {
		return nil
	}
	if s.PendingInit.Cwd == cwd {
		pi := s.PendingInit
		s.PendingInit = nil
		return pi
	}
	return nil
}

// =====================================================================
// INDEX CACHE OPERATIONS
// =====================================================================

// LoadIndex reads the cached session index from ~/.config/claix/index.json.
// Returns nil (not an error) if the file doesn't exist — the caller should
// treat this as "no cache available, need to sync".
func LoadIndex() (*IndexCache, error) {
	path, err := indexPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache yet — not an error
		}
		return nil, err
	}

	var idx IndexCache
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	return &idx, nil
}

// SaveIndex saves the scanned sessions to ~/.config/claix/index.json.
// This is called by `claix sync` after scanning all sessions.
// The UpdatedAt timestamp lets us show "last synced 5 minutes ago" in the TUI.
func SaveIndex(sessions []scanner.Session) error {
	dir, err := ensureConfigDir()
	if err != nil {
		return err
	}

	idx := IndexCache{
		Sessions:  sessions,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339), // ISO 8601 format
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "index.json")
	return os.WriteFile(path, data, 0o644)
}
