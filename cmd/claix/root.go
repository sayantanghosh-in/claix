package claix

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	// This imports our export package for generating markdown session summaries.
	"github.com/sayantanghosh-in/claix/internal/export"

	// This imports the MCP server — allows Claude Code to interact with claix mid-session
	// over JSON-RPC 2.0 on stdio.
	"github.com/sayantanghosh-in/claix/internal/mcp"

	// This imports our scanner package. In Go, the last segment of the import path
	// becomes the local name — so we use it as `scanner.ScanAll(...)`.
	"github.com/sayantanghosh-in/claix/internal/scanner"

	// This imports our store package — manages claix's own metadata (tags, notes, pins)
	// and the cached session index from `claix sync`.
	"github.com/sayantanghosh-in/claix/internal/store"

	// This imports our TUI package — the interactive Bubbletea interface.
	"github.com/sayantanghosh-in/claix/internal/tui"

	// Shared utilities (hyperlinks, browser opening)
	"github.com/sayantanghosh-in/claix/internal/util"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "claix",
	Short: "Smart TUI to search, organize, and resume your Claude Code sessions",
	Long: `claix — make your Claude sessions click.

A terminal UI for managing Claude Code sessions across all your projects.
Search, organize, tag, and resume any session from anywhere.`,
	Run: func(cmd *cobra.Command, args []string) {
		// First-run detection: if hooks aren't installed yet, offer to set them up.
		// This replaces the need for a separate `claix install` step for most users.
		if !hooksInstalled() {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Set up auto-sync hooks for Claude Code? (Y/n) ")
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer == "" || answer == "y" || answer == "yes" {
				installHooks()
				fmt.Println()
			}
		}

		// Load the saved theme before launching the TUI.
		// If no theme is set, ApplyTheme falls back to "default".
		if s, err := store.Load(); err == nil && s.Theme != "" {
			tui.ApplyTheme(s.Theme)
		}

		// Launch the interactive TUI.
		// tui.Run() blocks until the user quits (pressing 'q' or ctrl+c).
		// If the user selected a session to resume, Run() handles launching
		// `claude --resume <id>` before returning.
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of claix",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("claix", version)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Claude Code sessions across projects",
	Run: func(cmd *cobra.Command, args []string) {
		// Call our scanner to find all sessions.
		// "" means use the default ~/.claude path.
		sessions, err := scanner.ScanAll("")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error scanning sessions:", err)
			os.Exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return
		}

		// tabwriter aligns columns with tabs — like a spreadsheet for the terminal.
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

		fmt.Fprintln(w, "STATUS\tID\tPROJECT\tBRANCH\tLAST ACTIVE\tTITLE\tPRS")
		fmt.Fprintln(w, "------\t--\t-------\t------\t-----------\t-----\t---")

		for _, s := range sessions {
			// Skip empty sessions in CLI output too
			if s.Status == scanner.StatusEmpty {
				continue
			}

			// Status indicator
			status := "○"
			switch s.Status {
			case scanner.StatusActive:
				status = "●"
			case scanner.StatusDone:
				status = "✓"
			}

			shortID := s.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}

			project := shortenPath(s.ProjectPath)

			branch := s.GitBranch
			if branch == "" {
				branch = "-"
			}

			lastActive := s.LastActive
			if len(lastActive) >= 16 {
				lastActive = strings.Replace(lastActive[:16], "T", " ", 1)
			}

			title := s.Title
			if len(title) > 50 {
				title = title[:50] + "..."
			}

			// PR links
			var prParts []string
			for _, pr := range s.PRLinks {
				prParts = append(prParts, fmt.Sprintf("#%d", pr.Number))
			}
			prs := strings.Join(prParts, " ")
			if prs == "" {
				prs = "-"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				status, shortID, project, branch, lastActive, title, prs)
		}

		w.Flush()
		printProjectLink()
	},
}

// shortenPath takes a full path like "/Users/<user>/git/my-project"
// and returns the last 2 segments: "git/my-project".
// This keeps the output readable without wasting horizontal space.
// shortenPath returns the last 2 segments of a path, joined with " > "
// instead of "/" to prevent terminals from interpreting it as a URL.
func shortenPath(path string) string {
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) <= 3 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], " > ")
}

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Fuzzy search sessions by content or topic",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.ToLower(strings.Join(args, " "))

		sessions, err := scanner.ScanAll("")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error scanning sessions:", err)
			os.Exit(1)
		}

		// Load the store so we can also match against tags
		st, _ := store.Load()

		// Filter sessions that match the query in title, preview, branch, path, or tags
		var matched []scanner.Session
		for _, s := range sessions {
			if s.Status == scanner.StatusEmpty {
				continue
			}
			if strings.Contains(strings.ToLower(s.Title), query) ||
				strings.Contains(strings.ToLower(s.Preview), query) ||
				strings.Contains(strings.ToLower(s.GitBranch), query) ||
				strings.Contains(strings.ToLower(s.ProjectPath), query) {
				matched = append(matched, s)
				continue
			}
			// Check tags from the store
			if st != nil {
				meta := st.GetMeta(s.ID)
				for _, tag := range meta.Tags {
					if strings.Contains(strings.ToLower(tag), query) {
						matched = append(matched, s)
						break
					}
				}
			}
		}

		if len(matched) == 0 {
			fmt.Printf("No sessions found matching: %s\n", query)
			return
		}

		// Print results in the same tabwriter format as `claix list`
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "Found %d sessions matching \"%s\":\n\n", len(matched), query)
		fmt.Fprintln(w, "STATUS\tID\tPROJECT\tBRANCH\tLAST ACTIVE\tTITLE")
		fmt.Fprintln(w, "------\t--\t-------\t------\t-----------\t-----")

		for _, s := range matched {
			status := "○"
			switch s.Status {
			case scanner.StatusActive:
				status = "●"
			case scanner.StatusDone:
				status = "✓"
			}

			shortID := s.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}

			project := shortenPath(s.ProjectPath)

			branch := s.GitBranch
			if branch == "" {
				branch = "-"
			}

			lastActive := s.LastActive
			if len(lastActive) >= 16 {
				lastActive = strings.Replace(lastActive[:16], "T", " ", 1)
			}

			title := s.Title
			if len(title) > 50 {
				title = title[:50] + "..."
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				status, shortID, project, branch, lastActive, title)
		}
		w.Flush()
		printProjectLink()
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Pick and resume a session via interactive picker",
	Run: func(cmd *cobra.Command, args []string) {
		sessions, err := scanner.ScanAll("")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error scanning sessions:", err)
			os.Exit(1)
		}

		// Collect the 10 most recent non-empty sessions.
		// Sessions from ScanAll are already sorted by LastActive (newest first).
		var recent []scanner.Session
		for _, s := range sessions {
			if s.Status == scanner.StatusEmpty {
				continue
			}
			recent = append(recent, s)
			if len(recent) >= 10 {
				break
			}
		}

		if len(recent) == 0 {
			fmt.Println("No sessions found.")
			return
		}

		// Print a numbered list for the user to pick from
		fmt.Println("Recent sessions:")
		fmt.Println()
		for i, s := range recent {
			shortID := s.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			project := shortenPath(s.ProjectPath)
			title := s.Title
			if len(title) > 45 {
				title = title[:45] + "..."
			}
			status := "○"
			switch s.Status {
			case scanner.StatusActive:
				status = "●"
			case scanner.StatusDone:
				status = "✓"
			}
			fmt.Printf("  %s %2d. %s  %-20s  %s\n", status, i+1, shortID, project, title)
		}

		// Read the user's choice from stdin
		fmt.Printf("\nEnter number (1-%d) to resume, or q to quit: ", len(recent))
		var input string
		fmt.Scanln(&input)

		input = strings.TrimSpace(input)
		if input == "q" || input == "" {
			return
		}

		// Parse the number
		var choice int
		_, parseErr := fmt.Sscanf(input, "%d", &choice)
		if parseErr != nil || choice < 1 || choice > len(recent) {
			fmt.Fprintln(os.Stderr, "Invalid choice.")
			os.Exit(1)
		}

		// Launch claude --resume in the session's project directory
		selected := recent[choice-1]
		fmt.Printf("\nResuming session %s in %s...\n\n", selected.ID[:8], selected.ProjectPath)

		c := exec.Command("claude", "--resume", selected.ID)
		c.Dir = selected.ProjectPath
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if runErr := c.Run(); runErr != nil {
			fmt.Fprintln(os.Stderr, "Error resuming session:", runErr)
			os.Exit(1)
		}
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show session usage stats and activity heatmap",
	Run: func(cmd *cobra.Command, args []string) {
		// Load all sessions to compute session-level stats.
		sessions, err := scanner.ScanAll("")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error scanning sessions:", err)
			os.Exit(1)
		}

		// Load the stats cache for aggregate data (tokens, daily activity).
		stats, err := scanner.LoadStats("")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error loading stats:", err)
			os.Exit(1)
		}

		// ── Count sessions by status ──
		var activeCount, doneCount, emptyCount int
		for _, s := range sessions {
			switch s.Status {
			case scanner.StatusActive:
				activeCount++
			case scanner.StatusDone:
				doneCount++
			case scanner.StatusEmpty:
				emptyCount++
			}
		}
		totalSessions := len(sessions)

		// ── Count unique projects ──
		// A "project" is a unique ProjectPath (the working directory).
		projectSet := make(map[string]bool)
		for _, s := range sessions {
			if s.ProjectPath != "" {
				projectSet[s.ProjectPath] = true
			}
		}

		// ── Header ──
		fmt.Println("Claude Code Usage Stats")
		fmt.Println(strings.Repeat("\u2550", 23)) // ═ repeated
		fmt.Println()

		// ── Session/message/tool/project counts ──
		fmt.Printf("Sessions:  %d total  (%d active, %d done, %d empty)\n",
			totalSessions, activeCount, doneCount, emptyCount)
		fmt.Printf("Messages:  %s total\n", formatNumber(stats.TotalMessages))
		fmt.Printf("Tools:     %s calls\n", formatNumber(stats.TotalToolCalls))
		fmt.Printf("Projects:  %d\n", len(projectSet))

		// ── Activity sparkline (last 28 days) ──
		sparkData := stats.SparklineData(28)
		sparkline := scanner.RenderSparkline(sparkData)
		if sparkline != "" {
			fmt.Printf("\nActivity (last 28 days):\n%s\n", sparkline)
		}

		// ── Token usage by model ──
		if len(stats.TotalTokens) > 0 {
			fmt.Println("\nToken Usage:")

			// Sort models by token count (highest first) for consistent display.
			type modelTokens struct {
				name   string
				tokens int
			}
			var models []modelTokens
			for model, tokens := range stats.TotalTokens {
				models = append(models, modelTokens{name: model, tokens: tokens})
			}
			sort.Slice(models, func(i, j int) bool {
				return models[i].tokens > models[j].tokens
			})

			for _, m := range models {
				shortName := scanner.ModelShortName(m.name)
				fmt.Printf("  %-8s %s tokens\n", shortName, scanner.FormatTokenCount(m.tokens))
			}
		}

		// ── Top projects (by session count) ──
		// Count sessions per project, then show the top 5.
		projectCounts := make(map[string]int)
		for _, s := range sessions {
			if s.Status == scanner.StatusEmpty {
				continue // Don't count empty sessions
			}
			if s.ProjectPath != "" {
				projectCounts[s.ProjectPath]++
			}
		}

		if len(projectCounts) > 0 {
			type projectCount struct {
				path  string
				count int
			}
			var sorted []projectCount
			for path, count := range projectCounts {
				sorted = append(sorted, projectCount{path: path, count: count})
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].count > sorted[j].count
			})

			fmt.Println("\nTop Projects:")
			limit := 5
			if len(sorted) < limit {
				limit = len(sorted)
			}
			for i := 0; i < limit; i++ {
				p := sorted[i]
				display := shortenPath(p.path)
				// Right-align the session count for clean formatting.
				fmt.Printf("  %d. %-25s %2d sessions\n", i+1, display, p.count)
			}
		}

		// ── Recent sessions (last 5 non-empty) ──
		fmt.Println("\nRecent Sessions:")
		shown := 0
		for _, s := range sessions {
			if shown >= 5 {
				break
			}
			if s.Status == scanner.StatusEmpty {
				continue
			}

			shortID := s.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}

			project := shortenPath(s.ProjectPath)

			// Extract just the date portion (YYYY-MM-DD) from the timestamp.
			date := s.LastActive
			if len(date) >= 10 {
				date = date[:10]
			}

			// Status symbol
			statusIcon := "○"
			statusText := "Active"
			switch s.Status {
			case scanner.StatusDone:
				statusIcon = "✓"
				statusText = "Done"
			case scanner.StatusActive:
				statusIcon = "●"
				statusText = "Active"
			}

			fmt.Printf("  %s  %-20s %s  %s %s\n",
				shortID, project, date, statusIcon, statusText)
			shown++
		}
		printProjectLink()
	},
}

// formatNumber adds comma separators to large numbers for readability.
// 11734 → "11,734"
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Build the string with commas by processing groups of 3 digits.
	s := fmt.Sprintf("%d", n)
	var result strings.Builder

	// Calculate where the first group ends (1-3 digits before the first comma).
	firstGroup := len(s) % 3
	if firstGroup == 0 {
		firstGroup = 3
	}

	result.WriteString(s[:firstGroup])
	for i := firstGroup; i < len(s); i += 3 {
		result.WriteString(",")
		result.WriteString(s[i : i+3])
	}

	return result.String()
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync and index session data (used by hooks)",
	Run: func(cmd *cobra.Command, args []string) {
		// Step 1: Scan all Claude Code sessions from ~/.claude/projects/
		// scanner.ScanAll("") uses the default ~/.claude path.
		// This reads every .jsonl file and extracts metadata from each session.
		fmt.Print("Scanning sessions...")
		sessions, err := scanner.ScanAll("")
		if err != nil {
			fmt.Fprintln(os.Stderr, "\nError scanning sessions:", err)
			os.Exit(1)
		}

		// Step 2: Save the scanned sessions to the index cache at ~/.config/claix/index.json.
		// This allows the TUI to load instantly next time without re-scanning.
		if err := store.SaveIndex(sessions); err != nil {
			fmt.Fprintln(os.Stderr, "\nError saving index:", err)
			os.Exit(1)
		}

		// Step 3: Consume any pending init metadata (from `claix init`).
		// Match the pending init to the newest session in the same working directory.
		st, _ := store.Load()
		if st != nil && st.PendingInit != nil {
			cwd := st.PendingInit.Cwd
			pi := st.ConsumePendingInit(cwd)
			if pi != nil {
				// Find the newest session in the same cwd
				var newest *scanner.Session
				for i := range sessions {
					if sessions[i].ProjectPath == cwd {
						if newest == nil || sessions[i].LastActive > newest.LastActive {
							newest = &sessions[i]
						}
					}
				}
				if newest != nil {
					if pi.Title != "" {
						// Title is stored in the session's .jsonl by Claude, but we can
						// apply tags from the pending init to the store metadata.
					}
					for _, tag := range pi.Tags {
						st.AddTag(newest.ID, tag)
					}
					_ = st.Save()
					if len(pi.Tags) > 0 {
						fmt.Printf("\n  Applied tags %v to session %s\n", pi.Tags, newest.ID[:8])
					}
				}
			}
		}

		// Step 4: Count unique projects for the summary message.
		// We use a map as a set (map[string]bool) — adding a key twice is harmless.
		projectSet := make(map[string]bool)
		for _, s := range sessions {
			if s.ProjectPath != "" {
				projectSet[s.ProjectPath] = true
			}
		}

		fmt.Printf("\rSynced %d sessions across %d projects\n", len(sessions), len(projectSet))
		printProjectLink()
	},
}

var exportCmd = &cobra.Command{
	Use:   "export [session-id]",
	Short: "Export a session summary as markdown",
	Long: `Export generates a structured markdown summary of a Claude Code session.

The session-id argument can be a prefix (first 8 characters) — it will
match the first session whose ID starts with that prefix.

Output includes: metadata, conversation highlights, files changed, and PR links.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		prefix := args[0]

		// Scan all sessions to find the one matching the ID prefix.
		sessions, err := scanner.ScanAll("")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error scanning sessions:", err)
			os.Exit(1)
		}

		// Find the session whose ID starts with the given prefix.
		// This allows users to type just the first 8 chars instead of the full UUID.
		var matched *scanner.Session
		for i, s := range sessions {
			if strings.HasPrefix(s.ID, prefix) {
				matched = &sessions[i]
				break
			}
		}

		if matched == nil {
			fmt.Fprintf(os.Stderr, "No session found matching prefix: %s\n", prefix)
			os.Exit(1)
		}

		// Generate and print the markdown export to stdout.
		// Users can redirect this to a file: claix export abc123 > session.md
		markdown := export.Export(*matched)
		fmt.Print(markdown)
		printProjectLink()
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Set up Claude Code hooks and MCP integration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Setting up claix hooks for Claude Code...")
		fmt.Println()

		installHooks()

		fmt.Println()
		fmt.Println("MCP Integration:")
		fmt.Println("  To enable mid-session tagging, add to ~/.claude/settings.json:")
		fmt.Printf("  \"mcpServers\": { \"claix\": { \"command\": \"claix\", \"args\": [\"mcp-server\"] } }\n")
		fmt.Println()
		fmt.Println("Done! claix will auto-sync after sessions and show a context hint on startup.")
		printProjectLink()
	},
}

var themeCmd = &cobra.Command{
	Use:   "theme [name]",
	Short: "Set or view the TUI color theme",
	Long: `Set or view the TUI color theme.

Available themes: default, dracula, catppuccin, nord, gruvbox, tokyonight

Examples:
  claix theme              # Show current theme and list available themes
  claix theme dracula      # Switch to Dracula theme
  claix theme default      # Switch back to default theme`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load the store to read/write the theme setting.
		s, err := store.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error loading store:", err)
			os.Exit(1)
		}

		// No argument — show current theme and list available themes.
		if len(args) == 0 {
			current := s.Theme
			if current == "" {
				current = "default"
			}
			fmt.Printf("Current theme: %s\n\n", current)
			fmt.Println("Available themes:")
			for _, name := range tui.ThemeNames() {
				marker := "  "
				if name == current {
					marker = "▸ " // Arrow indicates current theme
				}
				// Show a color preview using the theme's primary + secondary colors
				theme := tui.Themes[name]
				primary := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Render("██")
				secondary := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Secondary)).Render("██")
				success := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Success)).Render("██")
				warning := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Warning)).Render("██")
				pr := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.PR)).Render("██")
				fmt.Printf("  %s%-12s %s %s %s %s %s\n", marker, name, primary, secondary, success, warning, pr)
			}
			fmt.Println("\nUsage: claix theme <name>")
			return
		}

		// Argument provided — set the theme.
		themeName := args[0]

		// Validate the theme name exists.
		if _, ok := tui.Themes[themeName]; !ok {
			fmt.Fprintf(os.Stderr, "Unknown theme: %s\n\nAvailable themes: %s\n",
				themeName, strings.Join(tui.ThemeNames(), ", "))
			os.Exit(1)
		}

		s.Theme = themeName
		if err := s.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "Error saving theme:", err)
			os.Exit(1)
		}

		fmt.Printf("Theme set to: %s\n", themeName)
		fmt.Println("Launch claix to see the new theme.")
		printProjectLink()
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove claix hooks from Claude Code settings",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Removing claix hooks from Claude Code...")
		fmt.Println()

		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: could not determine home directory:", err)
			os.Exit(1)
		}

		settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

		data, err := os.ReadFile(settingsPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("  No settings.json found — nothing to remove.")
				return
			}
			fmt.Fprintln(os.Stderr, "Error reading settings:", err)
			os.Exit(1)
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(data, &settings); err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing settings.json:", err)
			os.Exit(1)
		}

		// Remove claix entries from the Stop hooks array.
		hooks, ok := settings["hooks"].(map[string]interface{})
		if !ok {
			fmt.Println("  No hooks found — nothing to remove.")
			return
		}

		stopEntries, ok := hooks["Stop"].([]interface{})
		if !ok || len(stopEntries) == 0 {
			fmt.Println("  No Stop hooks found — nothing to remove.")
			return
		}

		// Filter out any entry that contains a "claix sync" command.
		var remaining []interface{}
		removed := 0
		for _, entry := range stopEntries {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				remaining = append(remaining, entry)
				continue
			}

			isClaix := false
			if innerHooks, ok := entryMap["hooks"].([]interface{}); ok {
				for _, h := range innerHooks {
					if hookMap, ok := h.(map[string]interface{}); ok {
						if hookMap["command"] == "claix sync" {
							isClaix = true
							break
						}
					}
				}
			}

			if isClaix {
				removed++
			} else {
				remaining = append(remaining, entry)
			}
		}

		if removed == 0 {
			fmt.Println("  No claix hooks found — nothing to remove.")
			return
		}

		// Update or remove the Stop key depending on what's left.
		if len(remaining) == 0 {
			delete(hooks, "Stop")
		} else {
			hooks["Stop"] = remaining
		}

		// If hooks map is now empty, remove it entirely for cleanliness.
		if len(hooks) == 0 {
			delete(settings, "hooks")
		} else {
			settings["hooks"] = hooks
		}

		out, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error encoding settings:", err)
			os.Exit(1)
		}

		if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "Error writing settings:", err)
			os.Exit(1)
		}

		fmt.Printf("  Removed %d claix hook(s) from Stop.\n", removed)

		// Also remove SessionStart hooks containing "claix context"
		startRemoved := 0
		if startEntries, ok := hooks["SessionStart"].([]interface{}); ok {
			var remaining2 []interface{}
			for _, entry := range startEntries {
				isClaix := false
				if entryMap, ok := entry.(map[string]interface{}); ok {
					if innerHooks, ok := entryMap["hooks"].([]interface{}); ok {
						for _, h := range innerHooks {
							if hookMap, ok := h.(map[string]interface{}); ok {
								if hookMap["command"] == "claix context" {
									isClaix = true
									break
								}
							}
						}
					}
				}
				if isClaix {
					startRemoved++
				} else {
					remaining2 = append(remaining2, entry)
				}
			}
			if len(remaining2) == 0 {
				delete(hooks, "SessionStart")
			} else {
				hooks["SessionStart"] = remaining2
			}
		}
		if startRemoved > 0 {
			fmt.Printf("  Removed %d claix hook(s) from SessionStart.\n", startRemoved)
		}

		if len(hooks) == 0 {
			delete(settings, "hooks")
		} else {
			settings["hooks"] = hooks
		}

		outBytes, marshalErr := json.MarshalIndent(settings, "", "  ")
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "Error encoding settings:", marshalErr)
			os.Exit(1)
		}
		if writeErr := os.WriteFile(settingsPath, outBytes, 0o644); writeErr != nil {
			fmt.Fprintln(os.Stderr, "Error writing settings:", writeErr)
			os.Exit(1)
		}

		fmt.Printf("  Settings updated: %s\n", settingsPath)
		fmt.Println()
		fmt.Println("Done! claix hooks have been removed from Claude Code.")
		printProjectLink()
	},
}

// mcpServerCmd runs claix as an MCP (Model Context Protocol) server.
// Claude Code launches this as a subprocess and communicates over stdin/stdout
// using JSON-RPC 2.0. This lets Claude call tools like "tag this session" or
// "list sessions" without the user leaving their conversation.
//
// To use: add claix to your Claude Code MCP settings:
//
//	{
//	  "mcpServers": {
//	    "claix": { "command": "claix", "args": ["mcp-server"] }
//	  }
//	}
var mcpServerCmd = &cobra.Command{
	Use:   "mcp-server",
	Short: "Run as MCP server for Claude Code integration",
	Long: `Run claix as a Model Context Protocol (MCP) server.

This command is not meant to be run directly — Claude Code starts it as a
subprocess and communicates over stdin/stdout using JSON-RPC 2.0.

Available tools:
  claix_tag_session    - Add a tag to a session
  claix_note_session   - Add a note to a session
  claix_list_sessions  - List recent sessions
  claix_session_info   - Get details about a session`,
	Run: func(cmd *cobra.Command, args []string) {
		mcp.Serve()
	},
}

// initCmd sets session metadata (title, tags) before launching Claude Code.
// This helps users organize sessions from the start instead of tagging later.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set session title and tags, then launch Claude Code",
	Long: `Set session metadata before starting a Claude Code session.

The title and tags are saved and automatically applied to the session
after it syncs. This helps you organize sessions from the start.

Examples:
  claix init                                    # Interactive prompts
  claix init --title "auth refactor"            # Just a title
  claix init --title "bugfix" --tags "urgent"   # Title + tags`,
	Run: func(cmd *cobra.Command, args []string) {
		title, _ := cmd.Flags().GetString("title")
		tagsStr, _ := cmd.Flags().GetString("tags")

		// Interactive mode if no flags provided
		if !cmd.Flags().Changed("title") && !cmd.Flags().Changed("tags") {
			reader := bufio.NewReader(os.Stdin)

			fmt.Print("Session title (optional): ")
			titleInput, _ := reader.ReadString('\n')
			title = strings.TrimSpace(titleInput)

			fmt.Print("Tags (comma-separated, optional): ")
			tagsInput, _ := reader.ReadString('\n')
			tagsStr = strings.TrimSpace(tagsInput)
		}

		// Parse tags from comma-separated string
		var tags []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		// Save pending init to store
		if title != "" || len(tags) > 0 {
			s, err := store.Load()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Warning: could not save session metadata:", err)
			} else {
				cwd, _ := os.Getwd()
				s.SetPendingInit(title, tags, cwd)
				_ = s.Save()
				if title != "" {
					fmt.Printf("Title: %s\n", title)
				}
				if len(tags) > 0 {
					fmt.Printf("Tags: %s\n", strings.Join(tags, ", "))
				}
				fmt.Println("Metadata saved. Starting Claude Code...")
			}
		} else {
			fmt.Println("No metadata set. Starting Claude Code...")
		}

		fmt.Println()

		// Launch Claude Code in the current directory
		claude := exec.Command("claude")
		claude.Stdin = os.Stdin
		claude.Stdout = os.Stdout
		claude.Stderr = os.Stderr
		if err := claude.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Error launching Claude Code:", err)
			os.Exit(1)
		}
	},
}

// contextCmd outputs a gentle hint for the SessionStart hook.
// This gets injected into Claude's context when a new session starts,
// nudging the user to describe what they're working on.
// Hidden from help since it's meant to be called by hooks, not users.
var contextCmd = &cobra.Command{
	Use:    "context",
	Short:  "Output session context hint (used by SessionStart hook)",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("[claix] Tip: You can say 'tag this session as <topic>' anytime to help organize your sessions later.")
	},
}

// hooksInstalled checks if claix hooks are already configured in Claude Code settings.
func hooksInstalled() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if err != nil {
		return false
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return false
	}
	stopEntries, ok := hooks["Stop"].([]interface{})
	if !ok {
		return false
	}
	for _, entry := range stopEntries {
		if entryMap, ok := entry.(map[string]interface{}); ok {
			if innerHooks, ok := entryMap["hooks"].([]interface{}); ok {
				for _, h := range innerHooks {
					if hookMap, ok := h.(map[string]interface{}); ok {
						if hookMap["command"] == "claix sync" {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// installHooks adds claix hooks to Claude Code settings.json.
// Called by both the first-run prompt and `claix install`.
func installHooks() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: could not determine home directory:", err)
		return
	}
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]interface{})
		} else {
			fmt.Fprintln(os.Stderr, "Error reading settings:", err)
			return
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing settings.json:", err)
			return
		}
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		hooks = make(map[string]interface{})
	}

	// Add Stop hook (claix sync)
	alreadyInstalled := false
	if stopEntries, ok := hooks["Stop"].([]interface{}); ok {
		for _, entry := range stopEntries {
			if entryMap, ok := entry.(map[string]interface{}); ok {
				if innerHooks, ok := entryMap["hooks"].([]interface{}); ok {
					for _, h := range innerHooks {
						if hookMap, ok := h.(map[string]interface{}); ok {
							if hookMap["command"] == "claix sync" {
								alreadyInstalled = true
							}
						}
					}
				}
			}
		}
	}

	if !alreadyInstalled {
		claixHookEntry := map[string]interface{}{
			"matcher": "",
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": "claix sync",
				},
			},
		}
		stopEntries, _ := hooks["Stop"].([]interface{})
		stopEntries = append(stopEntries, claixHookEntry)
		hooks["Stop"] = stopEntries
		fmt.Println("  Added 'Stop' hook: claix sync")
	}

	// Add SessionStart hook (claix context)
	startInstalled := false
	if startEntries, ok := hooks["SessionStart"].([]interface{}); ok {
		for _, entry := range startEntries {
			if entryMap, ok := entry.(map[string]interface{}); ok {
				if innerHooks, ok := entryMap["hooks"].([]interface{}); ok {
					for _, h := range innerHooks {
						if hookMap, ok := h.(map[string]interface{}); ok {
							if hookMap["command"] == "claix context" {
								startInstalled = true
							}
						}
					}
				}
			}
		}
	}

	if !startInstalled {
		sessionStartEntry := map[string]interface{}{
			"matcher": "startup",
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": "claix context",
				},
			},
		}
		startEntries, _ := hooks["SessionStart"].([]interface{})
		startEntries = append(startEntries, sessionStartEntry)
		hooks["SessionStart"] = startEntries
		fmt.Println("  Added 'SessionStart' hook: claix context")
	}

	if alreadyInstalled && startInstalled {
		fmt.Println("  Hooks already installed — nothing to do.")
		return
	}

	settings["hooks"] = hooks

	claudeDir := filepath.Join(homeDir, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error encoding settings:", err)
		return
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "Error writing settings:", err)
		return
	}
	fmt.Printf("  Settings updated: %s\n", settingsPath)
}

// printProjectLink prints the GitHub repo link at the end of CLI commands.
func printProjectLink() {
	link := util.MakeHyperlink(util.RepoURL, "github.com/sayantanghosh-in/claix")
	fmt.Printf("\n%s\n", link)
}

func init() {
	// Register init command flags
	initCmd.Flags().String("title", "", "Session title")
	initCmd.Flags().String("tags", "", "Comma-separated tags")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(themeCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(mcpServerCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
