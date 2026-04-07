package claix

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	// This imports our scanner package. In Go, the last segment of the import path
	// becomes the local name — so we use it as `scanner.ScanAll(...)`.
	"github.com/sayantanghosh-in/claix/internal/scanner"

	// This imports our TUI package — the interactive Bubbletea interface.
	"github.com/sayantanghosh-in/claix/internal/tui"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "claix",
	Short: "Smart TUI to search, organize, and resume your Claude Code sessions",
	Long: `claix — make your Claude sessions click.

A terminal UI for managing Claude Code sessions across all your projects.
Search, organize, tag, and resume any session from anywhere.`,
	Run: func(cmd *cobra.Command, args []string) {
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
		// TODO: Implement fuzzy search
		fmt.Println("searching for:", args[0])
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Pick and resume a session via interactive TUI",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Launch resume picker TUI
		fmt.Println("launching resume picker...")
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show session usage stats and activity heatmap",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement stats
		fmt.Println("showing stats...")
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync and index session data (used by hooks)",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement sync
		fmt.Println("syncing session data...")
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Set up Claude Code hooks and MCP integration",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement auto-install of hooks
		fmt.Println("installing hooks...")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(installCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
