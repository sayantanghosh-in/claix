package claix

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "claix",
	Short: "Smart TUI to search, organize, and resume your Claude Code sessions",
	Long: `claix — make your Claude sessions click.

A terminal UI for managing Claude Code sessions across all your projects.
Search, organize, tag, and resume any session from anywhere.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Launch TUI
		fmt.Println("claix", version, "— run 'claix --help' for usage")
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
		// TODO: Implement session listing
		fmt.Println("listing sessions...")
	},
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
