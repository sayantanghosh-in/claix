// Package tui contains the terminal user interface built with Bubbletea.
//
// lipgloss is like CSS for the terminal. You define styles (colors, borders,
// padding, alignment) and apply them to strings. Styles are immutable values —
// you chain methods to create new styles, similar to styled-components in React.
package tui

import "github.com/charmbracelet/lipgloss"

// =====================================================================
// COLOR PALETTE
// =====================================================================

var (
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple — brand color
	colorSecondary = lipgloss.Color("#06B6D4") // Cyan — accents, selected items
	colorMuted     = lipgloss.Color("#6B7280") // Gray — secondary text
	colorSuccess   = lipgloss.Color("#10B981") // Green — done status, branches
	colorWarning   = lipgloss.Color("#F59E0B") // Amber — active status, warnings
	colorText      = lipgloss.Color("#E5E7EB") // Light gray — main text
	colorDim       = lipgloss.Color("#4B5563") // Dark gray — very subtle text
	colorPR        = lipgloss.Color("#EC4899") // Pink — PR links (stand out)
	colorSparkline = lipgloss.Color("#818CF8") // Indigo — sparkline bars
)

// =====================================================================
// STYLES
// =====================================================================
//
// Each style is created with lipgloss.NewStyle() and chained with methods.
// .Render("text") applies the style and returns a styled string.
// Example: titleStyle.Render("CLAIX") → "\033[1;37;45m CLAIX \033[0m"
//          (bold white text on purple background, with padding)

var (
	// ── Title bar ──
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 2)

	taglineStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	// ── Session status indicators ──
	// ● for active (amber = needs attention)
	activeIndicator = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	// ✓ for done (green = completed)
	doneIndicator = lipgloss.NewStyle().
			Foreground(colorSuccess)

	// ── Session list styles ──
	selectedCursor = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	// The thick left border for the selected card (┃ vs │)
	selectedBorderStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(colorText)

	// ── Accent style for highlighted numbers ──
	accentStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	// ── Secondary info styles ──
	dimStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	branchStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	prStyle = lipgloss.NewStyle().
		Foreground(colorPR).
		Bold(true)

	previewStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	sparklineStyle = lipgloss.NewStyle().
			Foreground(colorSparkline)

	// ── Footer styles ──
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// ── Detail panel ──
	panelHeaderStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true)

	// ── Empty state ──
	emptyStyle = lipgloss.NewStyle().
			Foreground(colorWarning).
			Italic(true).
			Padding(2, 4)

	// ── Tag style (shown on cards and detail panel) ──
	tagStyle = lipgloss.NewStyle().
			Foreground(colorPR)

	// ── Search bar style (shown at bottom when searching) ──
	searchBarStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	// ── Generic input bar style (for tag/note input at bottom) ──
	inputBarStyle = lipgloss.NewStyle().
			Foreground(colorText)
)
