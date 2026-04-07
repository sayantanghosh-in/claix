package tui

// =====================================================================
// THEME SYSTEM
// =====================================================================
//
// Themes are named color palettes. Each theme defines 9 colors that
// control the entire look of the TUI. The styles in styles.go reference
// the package-level color variables (colorPrimary, colorSecondary, etc.),
// so switching a theme just means overwriting those variables and
// rebuilding the styles.
//
// Available themes:
//   - default   — purple + cyan (claix brand)
//   - dracula   — classic dark theme with purple/pink/green
//   - catppuccin — soft pastel dark theme (mocha variant)
//   - nord      — arctic blue palette
//   - gruvbox   — retro warm tones
//   - tokyonight — dark blue with vibrant accents
//
// To add a new theme: add a ThemeColors entry to the Themes map below.
// =====================================================================

import "github.com/charmbracelet/lipgloss"

// ThemeColors defines the 9 colors that control the entire TUI appearance.
// Think of this like a CSS custom properties object:
//
//	:root {
//	  --primary: #7C3AED;
//	  --secondary: #06B6D4;
//	  ...
//	}
type ThemeColors struct {
	Primary   string // Brand color — title bar background, main accent
	Secondary string // Selection, links, highlighted items
	Muted     string // Secondary text, labels, descriptions
	Success   string // Done status, branch names
	Warning   string // Active status, warnings
	Text      string // Main body text
	Dim       string // Very subtle text, borders
	PR        string // PR links, tags (needs to pop)
	Sparkline string // Activity sparkline bars
}

// Themes is the registry of all available themes.
// map[string]ThemeColors is like Record<string, ThemeColors> in TypeScript.
var Themes = map[string]ThemeColors{
	"default": {
		Primary:   "#7C3AED", // Purple
		Secondary: "#06B6D4", // Cyan
		Muted:     "#6B7280", // Gray
		Success:   "#10B981", // Green
		Warning:   "#F59E0B", // Amber
		Text:      "#E5E7EB", // Light gray
		Dim:       "#4B5563", // Dark gray
		PR:        "#EC4899", // Pink
		Sparkline: "#818CF8", // Indigo
	},
	"dracula": {
		Primary:   "#BD93F9", // Purple
		Secondary: "#8BE9FD", // Cyan
		Muted:     "#6272A4", // Comment gray
		Success:   "#50FA7B", // Green
		Warning:   "#FFB86C", // Orange
		Text:      "#F8F8F2", // Foreground
		Dim:       "#44475A", // Current line
		PR:        "#FF79C6", // Pink
		Sparkline: "#BD93F9", // Purple
	},
	"catppuccin": {
		Primary:   "#CBA6F7", // Mauve
		Secondary: "#89DCEB", // Sky
		Muted:     "#6C7086", // Overlay0
		Success:   "#A6E3A1", // Green
		Warning:   "#F9E2AF", // Yellow
		Text:      "#CDD6F4", // Text
		Dim:       "#45475A", // Surface0
		PR:        "#F5C2E7", // Pink
		Sparkline: "#B4BEFE", // Lavender
	},
	"nord": {
		Primary:   "#5E81AC", // Nord10
		Secondary: "#88C0D0", // Nord8
		Muted:     "#4C566A", // Nord3
		Success:   "#A3BE8C", // Nord14
		Warning:   "#EBCB8B", // Nord13
		Text:      "#ECEFF4", // Nord6
		Dim:       "#434C5E", // Nord2
		PR:        "#B48EAD", // Nord15
		Sparkline: "#81A1C1", // Nord9
	},
	"gruvbox": {
		Primary:   "#D3869B", // Purple
		Secondary: "#83A598", // Aqua
		Muted:     "#928374", // Gray
		Success:   "#B8BB26", // Green
		Warning:   "#FABD2F", // Yellow
		Text:      "#EBDBB2", // Light
		Dim:       "#504945", // Dark
		PR:        "#FB4934", // Red
		Sparkline: "#D3869B", // Purple
	},
	"tokyonight": {
		Primary:   "#7AA2F7", // Blue
		Secondary: "#7DCFFF", // Cyan
		Muted:     "#565F89", // Comment
		Success:   "#9ECE6A", // Green
		Warning:   "#E0AF68", // Yellow
		Text:      "#C0CAF5", // Foreground
		Dim:       "#3B4261", // Dark
		PR:        "#FF007C", // Magenta
		Sparkline: "#BB9AF7", // Purple
	},
}

// ThemeNames returns a sorted list of available theme names.
// Used by the `claix theme` command to show options.
func ThemeNames() []string {
	// We hardcode the order instead of sorting the map keys because
	// we want "default" to always be first.
	return []string{"default", "dracula", "catppuccin", "nord", "gruvbox", "tokyonight"}
}

// ApplyTheme sets the package-level color variables and rebuilds all styles.
// Call this before creating the Bubbletea program.
//
// If the theme name is not found, it silently falls back to "default".
func ApplyTheme(name string) {
	theme, ok := Themes[name]
	if !ok {
		theme = Themes["default"]
	}

	// Update the package-level color variables.
	// These are the same variables referenced by the style definitions in styles.go.
	colorPrimary = lipgloss.Color(theme.Primary)
	colorSecondary = lipgloss.Color(theme.Secondary)
	colorMuted = lipgloss.Color(theme.Muted)
	colorSuccess = lipgloss.Color(theme.Success)
	colorWarning = lipgloss.Color(theme.Warning)
	colorText = lipgloss.Color(theme.Text)
	colorDim = lipgloss.Color(theme.Dim)
	colorPR = lipgloss.Color(theme.PR)
	colorSparkline = lipgloss.Color(theme.Sparkline)

	// Rebuild all styles with the new colors.
	// In Go, lipgloss styles are values (not references), so we need to
	// reassign them. This is like re-creating styled-components with new theme props.
	rebuildStyles()
}

// rebuildStyles recreates all styles using the current color variables.
// Called after changing the theme colors.
func rebuildStyles() {
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(colorPrimary).
		Padding(0, 2)

	taglineStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Italic(true)

	activeIndicator = lipgloss.NewStyle().
		Foreground(colorWarning).
		Bold(true)

	doneIndicator = lipgloss.NewStyle().
		Foreground(colorSuccess)

	selectedCursor = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)

	selectedBorderStyle = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)

	normalItemStyle = lipgloss.NewStyle().
		Foreground(colorText)

	accentStyle = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)

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

	statusBarStyle = lipgloss.NewStyle().
		Foreground(colorDim)

	helpKeyStyle = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)

	helpDescStyle = lipgloss.NewStyle().
		Foreground(colorMuted)

	panelHeaderStyle = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)

	emptyStyle = lipgloss.NewStyle().
		Foreground(colorWarning).
		Italic(true).
		Padding(2, 4)

	tagStyle = lipgloss.NewStyle().
		Foreground(colorPR)

	searchBarStyle = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)

	inputBarStyle = lipgloss.NewStyle().
		Foreground(colorText)
}
