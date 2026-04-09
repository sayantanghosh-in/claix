package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sayantanghosh-in/claix/internal/scanner"
	"github.com/sayantanghosh-in/claix/internal/store"
	"github.com/sayantanghosh-in/claix/internal/util"
)

// =====================================================================
// BUBBLETEA — THE ELM ARCHITECTURE
// =====================================================================
//
// Init()   → runs once at startup, kicks off async data load
// Update() → handles events (keypresses, data loaded, terminal resize)
// View()   → renders the entire screen as a string
//
// The loop: Event → Update() → View() → render → wait for next event
// =====================================================================

// Model is the root application state.
type Model struct {
	sessions    []scanner.Session // Filtered sessions (no empty ones)
	allSessions []scanner.Session // Unfiltered (includes empty)
	stats       *scanner.Stats    // Aggregate usage stats
	cursor      int               // Which session card is highlighted
	width       int               // Terminal width
	height      int               // Terminal height
	quitting    bool              // User is exiting
	resuming    *scanner.Session  // Session to resume after TUI exits
	loaded      bool              // Data has finished loading
	panelScroll int               // Scroll offset for the right detail panel

	// Search mode — activated by pressing '/'
	searching   bool            // Whether we're in search mode
	searchQuery string          // Current search text (kept when pressing Enter)
	searchInput textinput.Model // Bubbles textinput component for search

	// Tag mode — activated by pressing 't' to add, 'x' to remove
	tagging     bool            // Whether we're adding a tag
	tagInput    textinput.Model // Textinput for entering a tag
	removingTag bool            // Whether we're removing a tag (press 'x')
	tagOptions  []string        // List of tags on the selected session (for removal picker)
	tagCursor   int             // Which tag is highlighted in the removal picker

	// Notes mode — activated by pressing 'n' on a session
	noting    bool            // Whether we're editing notes
	noteInput textinput.Model // Textinput for entering a note

	// Store — claix's own metadata (tags, notes, pins), loaded once at startup
	store *store.Store

	// Star nudge — shown periodically to encourage GitHub stars
	showStarNudge bool
}

// dataLoadedMsg is sent when session scan + stats load complete.
type dataLoadedMsg struct {
	sessions []scanner.Session
	stats    *scanner.Stats
	store    *store.Store
	err      error
}

func New() Model {
	// Initialize the search textinput
	si := textinput.New()
	si.Placeholder = "Search sessions..."
	si.CharLimit = 100
	si.Width = 40
	si.Prompt = "/ "

	// Initialize the tag textinput
	ti := textinput.New()
	ti.Placeholder = "Enter tag..."
	ti.CharLimit = 50
	ti.Width = 30
	ti.Prompt = "Tag: "

	// Initialize the note textinput
	ni := textinput.New()
	ni.Placeholder = "Enter note..."
	ni.CharLimit = 200
	ni.Width = 50
	ni.Prompt = "Note: "

	return Model{
		loaded:      false,
		searchInput: si,
		tagInput:    ti,
		noteInput:   ni,
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		sessions, err := scanner.ScanAll("")
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		stats, _ := scanner.LoadStats("")
		// Load claix's own store (tags, notes, pins)
		st, _ := store.Load()
		return dataLoadedMsg{sessions: sessions, stats: stats, store: st}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case dataLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.sessions = []scanner.Session{}
			m.allSessions = []scanner.Session{}
			return m, nil
		}
		m.allSessions = msg.sessions
		m.stats = msg.stats
		m.store = msg.store

		// Check if we should show the star nudge banner
		if m.store != nil {
			m.showStarNudge = m.store.ShouldShowStarNudge()
		}

		// Filter out empty sessions for the default view
		var filtered []scanner.Session
		for _, s := range msg.sessions {
			if s.Status != scanner.StatusEmpty {
				filtered = append(filtered, s)
			}
		}
		m.sessions = filtered
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// ── Search mode: forward keys to the search textinput ──
		if m.searching {
			switch msg.String() {
			case "esc":
				// Exit search, clear filter, restore full list
				m.searching = false
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.sessions = m.filterSessions("")
				m.cursor = 0
				m.panelScroll = 0
				return m, nil
			case "enter":
				// Keep the current filter and switch back to navigation mode
				m.searching = false
				m.searchQuery = m.searchInput.Value()
				return m, nil
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			default:
				// Let the textinput handle the key (typing, backspace, etc.)
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				// Filter sessions in real-time as the user types
				m.sessions = m.filterSessions(m.searchInput.Value())
				m.cursor = 0
				m.panelScroll = 0
				return m, cmd
			}
		}

		// ── Tag mode: forward keys to the tag textinput ──
		if m.tagging {
			switch msg.String() {
			case "esc":
				m.tagging = false
				m.tagInput.SetValue("")
				return m, nil
			case "enter":
				tag := strings.TrimSpace(m.tagInput.Value())
				if tag != "" && len(m.sessions) > 0 && m.cursor < len(m.sessions) && m.store != nil {
					sessionID := m.sessions[m.cursor].ID
					m.store.AddTag(sessionID, tag)
					_ = m.store.Save() // Persist to disk
				}
				m.tagging = false
				m.tagInput.SetValue("")
				return m, nil
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			default:
				var cmd tea.Cmd
				m.tagInput, cmd = m.tagInput.Update(msg)
				return m, cmd
			}
		}

		// ── Tag removal mode: pick a tag to remove ──
		if m.removingTag {
			switch msg.String() {
			case "esc", "q":
				m.removingTag = false
				return m, nil
			case "up", "k":
				if m.tagCursor > 0 {
					m.tagCursor--
				}
			case "down", "j":
				if m.tagCursor < len(m.tagOptions)-1 {
					m.tagCursor++
				}
			case "enter":
				// Remove the selected tag
				if m.tagCursor < len(m.tagOptions) && len(m.sessions) > 0 && m.cursor < len(m.sessions) && m.store != nil {
					sessionID := m.sessions[m.cursor].ID
					m.store.RemoveTag(sessionID, m.tagOptions[m.tagCursor])
					_ = m.store.Save()
				}
				m.removingTag = false
				return m, nil
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		// ── Notes mode: forward keys to the note textinput ──
		if m.noting {
			switch msg.String() {
			case "esc":
				m.noting = false
				m.noteInput.SetValue("")
				return m, nil
			case "enter":
				note := strings.TrimSpace(m.noteInput.Value())
				if note != "" && len(m.sessions) > 0 && m.cursor < len(m.sessions) && m.store != nil {
					sessionID := m.sessions[m.cursor].ID
					m.store.SetNotes(sessionID, note)
					_ = m.store.Save() // Persist to disk
				}
				m.noting = false
				m.noteInput.SetValue("")
				return m, nil
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			default:
				var cmd tea.Cmd
				m.noteInput, cmd = m.noteInput.Update(msg)
				return m, cmd
			}
		}

		// ── Normal navigation mode ──
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.panelScroll = 0
			}
		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				m.panelScroll = 0
			}
		case "right", "l", "shift+down":
			m.panelScroll++
		case "left", "h", "shift+up":
			if m.panelScroll > 0 {
				m.panelScroll--
			}
		case "/":
			// Enter search mode — focus the search textinput
			m.searching = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case "t":
			// Enter tag mode — focus the tag textinput
			if len(m.sessions) > 0 && m.cursor < len(m.sessions) {
				m.tagging = true
				m.tagInput.SetValue("")
				m.tagInput.Focus()
				return m, textinput.Blink
			}
		case "x":
			// Enter tag removal mode — show a picker of existing tags
			if len(m.sessions) > 0 && m.cursor < len(m.sessions) && m.store != nil {
				meta := m.store.GetMeta(m.sessions[m.cursor].ID)
				if len(meta.Tags) > 0 {
					m.removingTag = true
					m.tagOptions = meta.Tags
					m.tagCursor = 0
				}
			}
		case "n":
			// Enter notes mode — focus the note textinput
			if len(m.sessions) > 0 && m.cursor < len(m.sessions) {
				m.noting = true
				// Pre-fill with existing note if any
				if m.store != nil {
					meta := m.store.GetMeta(m.sessions[m.cursor].ID)
					m.noteInput.SetValue(meta.Notes)
				}
				m.noteInput.Focus()
				return m, textinput.Blink
			}
		case "enter":
			if len(m.sessions) > 0 && m.cursor < len(m.sessions) {
				selected := m.sessions[m.cursor]
				m.resuming = &selected
				m.quitting = true
				return m, tea.Quit
			}
		case "s":
			// Star nudge: open GitHub repo in browser and hide permanently
			if m.showStarNudge && m.store != nil {
				_ = util.OpenBrowser(util.RepoURL)
				m.store.MarkStarred()
				_ = m.store.Save()
				m.showStarNudge = false
			}
		case "d":
			// Star nudge: dismiss for 10 days
			if m.showStarNudge && m.store != nil {
				m.store.DismissStarNudge()
				_ = m.store.Save()
				m.showStarNudge = false
			}
		}
	}

	return m, nil
}

// filterSessions returns non-empty sessions matching the query string.
// It checks title, preview, git branch, project path, and tags (from the store).
// If query is empty, all non-empty sessions are returned.
func (m Model) filterSessions(query string) []scanner.Session {
	var result []scanner.Session
	q := strings.ToLower(strings.TrimSpace(query))

	for _, s := range m.allSessions {
		if s.Status == scanner.StatusEmpty {
			continue
		}
		// If no query, include all non-empty sessions
		if q == "" {
			result = append(result, s)
			continue
		}
		// Check each searchable field for a substring match
		if strings.Contains(strings.ToLower(s.Title), q) ||
			strings.Contains(strings.ToLower(s.Preview), q) ||
			strings.Contains(strings.ToLower(s.GitBranch), q) ||
			strings.Contains(strings.ToLower(s.ProjectPath), q) {
			result = append(result, s)
			continue
		}
		// Also check tags from the store
		if m.store != nil {
			meta := m.store.GetMeta(s.ID)
			for _, tag := range meta.Tags {
				if strings.Contains(strings.ToLower(tag), q) {
					result = append(result, s)
					break
				}
			}
		}
	}
	return result
}

// =====================================================================
// VIEW — 2-COLUMN LAYOUT
// =====================================================================
//
// Layout:
//   ┌──────────────────────────────────────────────┬─────────────────────┐
//   │ CLAIX  make your Claude sessions click       │                     │
//   │                                              │                     │
//   │ 53 sessions  ● 3 active  ✓ 36 done          │  Session Detail     │
//   │                                              │                     │
//   │ ▁▃▇▅▂▆█▃  28d | 11.7k msgs                  │  c8a4f03f           │
//   │                                              │  Documents/personal │
//   │ another-project ████ 13  my-project ███ 7      │  Branch: HEAD       │
//   │ ────────────────────────────────────────      │  2026-04-07 07:52   │
//   │                                              │                     │
//   │  ┃ ✓ c8a4f03f  Documents/personal   Apr 7   │  Great idea. This   │
//   │  ┃   Great idea.                             │  is a much better   │
//   │  ┃   10 read · 26 edited                    │  approach. Let me   │
//   │                                              │  scaffold the...    │
//   │  │ ✓ 61f0b6b2  <user>/git           Apr 7   │                     │
//   │  │   Great list!                             │  Status: ✓ Done     │
//   │  │   35 read · 5 edited                     │  10 read · 26 edit  │
//   │                                              │                     │
//   │ 1-7 of 39 sessions (empty hidden)            │  Repos touched:     │
//   │ ↑↓ navigate  enter resume  q quit            │   personal (22)     │
//   └──────────────────────────────────────────────┴─────────────────────┘

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// ── Empty state: show a centered landing screen ──
	if m.loaded && len(m.allSessions) == 0 {
		return m.renderEmptyState()
	}

	// Calculate column widths: 3:1 ratio
	rightWidth := m.width / 4
	if rightWidth < 28 { rightWidth = 28 }
	if rightWidth > 50 { rightWidth = 50 }
	leftWidth := m.width - rightWidth - 3

	// Build left and right columns separately, then merge line by line
	leftLines := m.renderLeftColumn(leftWidth)
	allRightLines := m.renderRightColumn(rightWidth)

	// Apply scroll offset to right panel
	maxScroll := len(allRightLines) - m.height + 2
	if maxScroll < 0 { maxScroll = 0 }
	if m.panelScroll > maxScroll { m.panelScroll = maxScroll }
	rightLines := allRightLines
	if m.panelScroll > 0 && m.panelScroll < len(allRightLines) {
		rightLines = allRightLines[m.panelScroll:]
	}

	var b strings.Builder

	// Only render divider for lines where at least one column has content.
	// This prevents the divider from trailing into empty space below the content.
	contentLines := len(leftLines)
	if len(rightLines) > contentLines { contentLines = len(rightLines) }

	// Reserve last 2 lines for the footer (rendered separately, full width, no divider)
	maxContentLines := m.height - 2
	if maxContentLines < contentLines { contentLines = maxContentLines }

	divider := dimStyle.Render("│")

	for i := 0; i < contentLines; i++ {
		left := ""
		if i < len(leftLines) { left = leftLines[i] }
		right := ""
		if i < len(rightLines) { right = rightLines[i] }

		vw := lipgloss.Width(left)
		pad := leftWidth - vw
		if pad < 0 { pad = 0 }

		b.WriteString(left)
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(" " + divider + " ")
		b.WriteString(right)
		b.WriteString("\n")
	}

	// ── Footer: rendered full-width, no divider ──
	b.WriteString(fmt.Sprintf("  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s\n",
		helpKeyStyle.Render("↑↓"), helpDescStyle.Render("navigate"),
		helpKeyStyle.Render("←→"), helpDescStyle.Render("scroll"),
		helpKeyStyle.Render("/"), helpDescStyle.Render("search"),
		helpKeyStyle.Render("t"), helpDescStyle.Render("tag"),
		helpKeyStyle.Render("x"), helpDescStyle.Render("untag"),
		helpKeyStyle.Render("n"), helpDescStyle.Render("note"),
		helpKeyStyle.Render("enter"), helpDescStyle.Render("resume"),
		helpKeyStyle.Render("q"), helpDescStyle.Render("quit"),
	))

	// ── Bottom bar: show search/tag/note input when active ──
	if m.searching {
		b.WriteString(searchBarStyle.Render(" " + m.searchInput.View()))
		b.WriteString("\n")
	} else if m.tagging {
		b.WriteString(inputBarStyle.Render(" " + m.tagInput.View()))
		b.WriteString("\n")
	} else if m.noting {
		b.WriteString(inputBarStyle.Render(" " + m.noteInput.View()))
		b.WriteString("\n")
	} else if m.removingTag {
		// Show a tag picker — user selects which tag to remove
		b.WriteString(inputBarStyle.Render(" Remove tag (↑↓ select, enter remove, esc cancel):"))
		b.WriteString("\n")
		for i, tag := range m.tagOptions {
			if i == m.tagCursor {
				b.WriteString("   " + selectedItemStyle.Render("▸ "+tag))
			} else {
				b.WriteString("     " + dimStyle.Render(tag))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// =====================================================================
// LEFT COLUMN — Header + Session Cards
// =====================================================================

// renderEmptyState shows a centered landing page when no sessions exist.
// Used for fresh installs or when running on a machine without Claude Code sessions.
func (m Model) renderEmptyState() string {
	logo := `
 ██████╗ ██╗       █████╗  ██╗ ██╗  ██╗
██╔════╝ ██║      ██╔══██╗ ██║  ██╗██╔╝
██║      ██║      ███████║ ██║   ████╔╝
██║      ██║      ██╔══██║ ██║  ██╔╝██╗
╚██████╗ ███████╗ ██║  ██║ ██║ ██╔╝  ██╗
 ╚═════╝ ╚══════╝ ╚═╝  ╚═╝ ╚═╝ ╚═╝  ╚═╝`

	githubLink := makeHyperlink(util.RepoURL, accentStyle.Render("github.com/sayantanghosh-in/claix"))
	websiteLink := makeHyperlink("https://sayantanghosh.in", accentStyle.Render("sayantanghosh.in"))

	divider := dimStyle.Render("─────────────────────────────────────────")

	content := accentStyle.Render(logo) + "\n\n" +
		taglineStyle.Render("make your Claude sessions click") + "\n\n" +
		divider + "\n\n" +
		normalItemStyle.Render("No Claude Code sessions found yet.") + "\n\n" +
		dimStyle.Render("Quick Start:") + "\n" +
		normalItemStyle.Render("  1. Run ") + accentStyle.Render("claude") + normalItemStyle.Render(" to create your first session") + "\n" +
		normalItemStyle.Render("  2. Run ") + accentStyle.Render("claix") + normalItemStyle.Render(" to browse your sessions") + "\n\n" +
		divider + "\n\n" +
		dimStyle.Render("Author:  ") + normalItemStyle.Render("Sayantan Ghosh") + "\n" +
		dimStyle.Render("GitHub:  ") + githubLink + "\n" +
		dimStyle.Render("Website: ") + websiteLink + "\n\n" +
		dimStyle.Render("Press q to quit")

	// Center the content on screen using lipgloss.Place
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderLeftColumn(width int) []string {
	var lines []string

	// ── Header: compact ASCII art + links ──
	logoLine1 := accentStyle.Render("█▀▀ █   ▄▀▄ █ ▀▄▀")
	logoLine2 := accentStyle.Render("█▄▄ █▄▄ █▀█ █ █ █")
	githubLink := makeHyperlink(util.RepoURL, dimStyle.Render("github.com/sayantanghosh-in/claix"))
	websiteLink := makeHyperlink("https://sayantanghosh.in", dimStyle.Render("sayantanghosh.in"))
	lines = append(lines, " "+logoLine1+"   "+githubLink)
	lines = append(lines, " "+logoLine2+"   "+websiteLink)

	if !m.loaded {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("  Scanning sessions..."))
		return lines
	}

	if len(m.sessions) == 0 {
		lines = append(lines, "")
		lines = append(lines, emptyStyle.Render("No Claude Code sessions found."))
		return lines
	}

	// Star nudge banner (shown periodically)
	if m.showStarNudge {
		lines = append(lines, starBannerStyle.Render(
			"  ⭐ Enjoying claix? Star us on GitHub!  ") +
			helpKeyStyle.Render("s") + helpDescStyle.Render(" star  ") +
			helpKeyStyle.Render("d") + helpDescStyle.Render(" dismiss"))
	}

	lines = append(lines, "") // Spacing

	// ── Stats bar ──
	activeCount, doneCount, emptyCount := 0, 0, 0
	projectSet := make(map[string]bool)
	for _, s := range m.allSessions {
		switch s.Status {
		case scanner.StatusActive:
			activeCount++
		case scanner.StatusDone:
			doneCount++
		case scanner.StatusEmpty:
			emptyCount++
		}
		projectSet[s.ProjectPath] = true
	}

	statsLine := fmt.Sprintf("  %s sessions  %s %d active  %s %d done  %s %d empty  │  %d projects",
		accentStyle.Render(fmt.Sprintf("%d", len(m.allSessions))),
		activeIndicator.Render("●"), activeCount,
		doneIndicator.Render("✓"), doneCount,
		dimStyle.Render("○"), emptyCount,
		len(projectSet),
	)
	lines = append(lines, statsLine)
	lines = append(lines, "") // Spacing

	// ── Sparkline ──
	if m.stats != nil && len(m.stats.DailyActivity) > 0 {
		sparkData := m.stats.SparklineData(28)
		sparkline := scanner.RenderSparkline(sparkData)

		var tokenParts []string
		var modelNames []string
		for model := range m.stats.TotalTokens {
			modelNames = append(modelNames, model)
		}
		sort.Strings(modelNames)
		for _, model := range modelNames {
			tokens := m.stats.TotalTokens[model]
			tokenParts = append(tokenParts, fmt.Sprintf("%s %s",
				scanner.ModelShortName(model),
				scanner.FormatTokenCount(tokens),
			))
		}

		sparkLine := fmt.Sprintf("  %s  28d  │  %s msgs  %s tools",
			sparklineStyle.Render(sparkline),
			scanner.FormatTokenCount(m.stats.TotalMessages),
			scanner.FormatTokenCount(m.stats.TotalToolCalls),
		)
		if len(tokenParts) > 0 {
			sparkLine += fmt.Sprintf("  │  %s", strings.Join(tokenParts, "  "))
		}
		lines = append(lines, sparkLine)
		lines = append(lines, "") // Spacing
	}

	// ── Top projects ──
	projectCounts := make(map[string]int)
	for _, s := range m.allSessions {
		if s.Status != scanner.StatusEmpty {
			projectCounts[shortenPath(s.ProjectPath)]++
		}
	}
	if len(projectCounts) > 0 {
		type pc struct{ name string; count int }
		var sorted []pc
		for name, count := range projectCounts {
			sorted = append(sorted, pc{name, count})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

		var topParts []string
		maxCount := sorted[0].count
		for i, p := range sorted {
			if i >= 3 { break }
			barLen := p.count * 8 / maxCount
			if barLen < 1 { barLen = 1 }
			bar := strings.Repeat("█", barLen) + strings.Repeat("░", 8-barLen)
			topParts = append(topParts, fmt.Sprintf("%s %s %d",
				accentStyle.Render(p.name), dimStyle.Render(bar), p.count))
		}
		lines = append(lines, "  "+strings.Join(topParts, "    "))
	}

	// ── Separator ──
	lines = append(lines, "")
	sepWidth := width - 4
	if sepWidth < 30 { sepWidth = 30 }
	lines = append(lines, "  "+dimStyle.Render(strings.Repeat("─", sepWidth)))
	lines = append(lines, "")

	// ── Session cards (dynamic height) ──
	// Pre-compute all card line groups so we can paginate by cumulative line count.
	// Each card has 3 base lines + optional tag lines + 1 blank separator.
	type cardLines struct {
		lines []string
	}

	allCards := make([]cardLines, len(m.sessions))
	for i, s := range m.sessions {
		isSelected := i == m.cursor

		border := dimStyle.Render("│")
		if isSelected { border = selectedBorderStyle.Render("┃") }

		// Status icon
		statusIcon := dimStyle.Render("○")
		switch s.Status {
		case scanner.StatusActive: statusIcon = activeIndicator.Render("●")
		case scanner.StatusDone:   statusIcon = doneIndicator.Render("✓")
		}

		shortID := s.ID
		if len(shortID) > 8 { shortID = shortID[:8] }

		project := shortenPath(s.ProjectPath)
		if len(project) > 20 { project = project[:20] + ".." }

		// Truncate branch for display
		branchTrunc := s.GitBranch
		if branchTrunc == "HEAD" { branchTrunc = "" }
		if len(branchTrunc) > 18 { branchTrunc = branchTrunc[:18] + ".." }

		// Date + time
		lastActive := s.LastActive
		if len(lastActive) >= 16 {
			lastActive = strings.Replace(lastActive[:16], "T", " ", 1)
		}

		var cl []string

		// Line 1: status + id + project + branch + datetime (inline)
		var fullLine string
		if isSelected {
			styledContent := fmt.Sprintf("%s %s  %s%s",
				statusIcon,
				accentStyle.Render(shortID),
				selectedItemStyle.Render(project),
				func() string { if branchTrunc != "" { return "  " + branchStyle.Render(branchTrunc) }; return "" }(),
			)
			fullLine = fmt.Sprintf(" %s %s %s",
				selectedCursor.Render("▸"), border, styledContent)
		} else {
			styledContent := fmt.Sprintf("%s %s  %s%s",
				statusIcon,
				dimStyle.Render(shortID),
				project,
				func() string { if branchTrunc != "" { return "  " + branchStyle.Render(branchTrunc) }; return "" }(),
			)
			fullLine = fmt.Sprintf("   %s %s", border, styledContent)
		}
		cl = append(cl, fullLine+"  "+dimStyle.Render(lastActive))

		// Line 2: title — if the session has PRs, make the PR number a clickable hyperlink
		titleText := s.Title
		maxW := width - 12
		if maxW < 20 { maxW = 20 }
		if len(titleText) > maxW { titleText = titleText[:maxW] + "..." }

		titleRendered := titleText
		if len(s.PRLinks) > 0 {
			pr := s.PRLinks[0]
			url := pr.URL
			if url == "" {
				url = fmt.Sprintf("https://github.com/%s/pull/%d", pr.Repository, pr.Number)
			}
			prLabel := fmt.Sprintf("PR #%d", pr.Number)
			if isSelected {
				titleRendered = makeHyperlink(url, selectedItemStyle.Render(prLabel))
			} else {
				titleRendered = makeHyperlink(url, prStyle.Render(prLabel))
			}
			repoName := pr.Repository
			if idx := strings.LastIndex(repoName, "/"); idx >= 0 {
				repoName = repoName[idx+1:]
			}
			suffix := " — " + repoName
			if isSelected {
				titleRendered += selectedItemStyle.Render(suffix)
			} else {
				titleRendered += normalItemStyle.Render(suffix)
			}
		} else if isSelected {
			titleRendered = selectedItemStyle.Render(titleText)
		} else {
			titleRendered = normalItemStyle.Render(titleText)
		}

		if isSelected {
			cl = append(cl, fmt.Sprintf("     %s   %s", border, titleRendered))
		} else {
			cl = append(cl, fmt.Sprintf("   %s   %s", border, titleRendered))
		}

		// Line 3: file activity (no tags — tags go on their own line)
		var actParts []string
		readCount := len(s.Activity.FilesRead)
		editCount := len(s.Activity.FilesEdited)
		if readCount > 0 || editCount > 0 {
			actParts = append(actParts, fmt.Sprintf("%d read · %d edited", readCount, editCount))
		}
		if s.Activity.RepoSummary != "" {
			summary := s.Activity.RepoSummary
			if len(summary) > 35 { summary = summary[:35] + "..." }
			actParts = append(actParts, summary)
		}
		actLine := strings.Join(actParts, "  │  ")
		if actLine != "" {
			cl = append(cl, fmt.Sprintf("   %s   %s", border, dimStyle.Render(actLine)))
		} else {
			cl = append(cl, fmt.Sprintf("   %s", border))
		}

		// Line 4+: tags on separate line(s) with word wrapping
		if m.store != nil {
			meta := m.store.GetMeta(s.ID)
			if len(meta.Tags) > 0 {
				var tagStrs []string
				for _, tag := range meta.Tags {
					tagStrs = append(tagStrs, "#"+tag)
				}
				tagText := strings.Join(tagStrs, "  ")
				tagWidth := width - 10
				if tagWidth < 20 { tagWidth = 20 }
				wrapped := wordWrap(tagText, tagWidth)
				for _, wl := range wrapped {
					cl = append(cl, fmt.Sprintf("   %s   %s", border, tagStyle.Render(wl)))
				}
			}
		}

		allCards[i] = cardLines{lines: cl}
	}

	// Paginate by cumulative line count (dynamic card heights)
	headerLineCount := len(lines)
	footerLines := 3
	availableLines := m.height - headerLineCount - footerLines

	// Find the start card: scroll so the cursor card is visible
	start := 0
	// First, try to show as many cards as fit, with cursor visible
	for {
		totalLines := 0
		cursorVisible := false
		for j := start; j < len(m.sessions); j++ {
			cardH := len(allCards[j].lines) + 1 // +1 for blank separator
			if totalLines+cardH > availableLines && j > start {
				break
			}
			totalLines += cardH
			if j == m.cursor { cursorVisible = true }
		}
		if cursorVisible || start >= m.cursor {
			break
		}
		start++
	}

	// Render visible cards
	end := start
	totalUsed := 0
	for j := start; j < len(m.sessions); j++ {
		cardH := len(allCards[j].lines) + 1
		if totalUsed+cardH > availableLines && j > start {
			break
		}
		totalUsed += cardH
		end = j + 1
	}

	for i := start; i < end; i++ {
		lines = append(lines, allCards[i].lines...)
		if i < end-1 { lines = append(lines, "") }
	}

	// Scroll info (part of left column content, above the footer)
	visibleCards := end - start
	if len(m.sessions) > visibleCards {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  %d-%d of %d sessions (empty hidden)", start+1, end, len(m.sessions))))
	}

	return lines
}

// =====================================================================
// RIGHT COLUMN — Detail Panel for Selected Session
// =====================================================================

func (m Model) renderRightColumn(width int) []string {
	var lines []string

	// Panel header
	lines = append(lines, panelHeaderStyle.Render("  Session Detail"))
	lines = append(lines, dimStyle.Render("  "+strings.Repeat("─", width-4)))
	lines = append(lines, "")

	if !m.loaded || len(m.sessions) == 0 || m.cursor >= len(m.sessions) {
		lines = append(lines, dimStyle.Render("  No session selected"))
		return lines
	}

	s := m.sessions[m.cursor]

	// Wrap width for text content in the panel
	wrapWidth := width - 4
	if wrapWidth < 15 { wrapWidth = 15 }

	// ── Session ID ──
	shortID := s.ID
	if len(shortID) > 8 { shortID = shortID[:8] }
	lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("ID:"), accentStyle.Render(shortID)))

	// ── Project (word wrap long paths) ──
	projectPath := shortenPath(s.ProjectPath)
	if len(projectPath) > wrapWidth-10 {
		wrapped := wordWrap(projectPath, wrapWidth-10)
		for i, wl := range wrapped {
			if i == 0 {
				lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Project:"), wl))
			} else {
				lines = append(lines, fmt.Sprintf("           %s", wl))
			}
		}
	} else {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Project:"), projectPath))
	}

	// ── Branch (word wrap long branch names) ──
	if s.GitBranch != "" && s.GitBranch != "HEAD" {
		branchName := s.GitBranch
		if len(branchName) > wrapWidth-9 {
			wrapped := wordWrap(branchName, wrapWidth-9)
			for i, wl := range wrapped {
				if i == 0 {
					lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Branch:"), branchStyle.Render(wl)))
				} else {
					lines = append(lines, fmt.Sprintf("          %s", branchStyle.Render(wl)))
				}
			}
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Branch:"), branchStyle.Render(branchName)))
		}
	}

	// ── Status ──
	statusText := dimStyle.Render("○ Empty")
	switch s.Status {
	case scanner.StatusActive:
		statusText = activeIndicator.Render("● Active")
	case scanner.StatusDone:
		statusText = doneIndicator.Render("✓ Done")
	}
	lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Status:"), statusText))

	// ── Time ──
	started := s.StartedAt
	if len(started) >= 16 { started = strings.Replace(started[:16], "T", " ", 1) }
	lastActive := s.LastActive
	if len(lastActive) >= 16 { lastActive = strings.Replace(lastActive[:16], "T", " ", 1) }
	lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Started:"), started))
	lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Last:"), lastActive))

	// ── Messages ──
	lines = append(lines, fmt.Sprintf("  %s %d (%d you, %d Claude)",
		dimStyle.Render("Msgs:"), s.TotalMsgs, s.UserMsgs, s.AssistantMsgs))

	// ── Slug (word wrap if long) ──
	if s.Slug != "" {
		if len(s.Slug) > wrapWidth-7 {
			wrapped := wordWrap(s.Slug, wrapWidth-7)
			for i, wl := range wrapped {
				if i == 0 {
					lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Slug:"), wl))
				} else {
					lines = append(lines, fmt.Sprintf("         %s", wl))
				}
			}
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Slug:"), s.Slug))
		}
	}

	// ── Version ──
	if s.Version != "" {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("CLI:"), s.Version))
	}

	// ── PR Links (show early — most actionable info) ──
	if len(s.PRLinks) > 0 {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("  "+strings.Repeat("─", width-4)))
		lines = append(lines, "")
		lines = append(lines, panelHeaderStyle.Render("  Pull Requests"))
		lines = append(lines, "")

		// Deduplicate PRs
		seen := make(map[string]bool)
		for _, pr := range s.PRLinks {
			url := pr.URL
			if url == "" {
				url = fmt.Sprintf("https://github.com/%s/pull/%d", pr.Repository, pr.Number)
			}
			if seen[url] { continue }
			seen[url] = true

			// Show as: org > repo > PR #N (clickable)
			// Split "org/repo" into separate parts joined with " > "
			org := pr.Repository
			repo := ""
			if idx := strings.Index(pr.Repository, "/"); idx >= 0 {
				org = pr.Repository[:idx]
				repo = pr.Repository[idx+1:]
			}

			prLabel := fmt.Sprintf("PR #%d", pr.Number)
			clickable := makeHyperlink(url, prStyle.Render(prLabel))

			if repo != "" {
				lines = append(lines, fmt.Sprintf("  %s > %s > %s",
					dimStyle.Render(org), dimStyle.Render(repo), clickable))
			} else {
				lines = append(lines, fmt.Sprintf("  %s > %s",
					dimStyle.Render(org), clickable))
			}
		}
	}

	// ── File Activity ──
	readCount := len(s.Activity.FilesRead)
	editCount := len(s.Activity.FilesEdited)
	if readCount > 0 || editCount > 0 {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("  "+strings.Repeat("─", width-4)))
		lines = append(lines, "")
		lines = append(lines, panelHeaderStyle.Render("  Files"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %s %d  %s %d",
			dimStyle.Render("Read:"), readCount,
			dimStyle.Render("Edited:"), editCount))

		if s.Activity.RepoSummary != "" {
			lines = append(lines, "")
			lines = append(lines, dimStyle.Render("  Repos touched:"))
			repos := strings.Split(s.Activity.RepoSummary, ", ")
			for _, repo := range repos {
				if len(repo) > wrapWidth-3 {
					wrapped := wordWrap(repo, wrapWidth-3)
					for _, wl := range wrapped {
						lines = append(lines, "   "+normalItemStyle.Render(wl))
					}
				} else {
					lines = append(lines, "   "+normalItemStyle.Render(repo))
				}
			}
		}
	}

	// ── Tags (from claix store) ──
	if m.store != nil {
		meta := m.store.GetMeta(s.ID)
		if len(meta.Tags) > 0 {
			lines = append(lines, "")
			lines = append(lines, dimStyle.Render("  "+strings.Repeat("─", width-4)))
			lines = append(lines, "")
			lines = append(lines, panelHeaderStyle.Render("  Tags"))
			lines = append(lines, "")
			var tagStrs []string
			for _, tag := range meta.Tags {
				tagStrs = append(tagStrs, tagStyle.Render("#"+tag))
			}
			lines = append(lines, "  "+strings.Join(tagStrs, "  "))
		}

		// ── Notes (from claix store) ──
		if meta.Notes != "" {
			lines = append(lines, "")
			lines = append(lines, dimStyle.Render("  "+strings.Repeat("─", width-4)))
			lines = append(lines, "")
			lines = append(lines, panelHeaderStyle.Render("  Notes"))
			lines = append(lines, "")
			wrapped := wordWrap(meta.Notes, wrapWidth)
			for _, wl := range wrapped {
				lines = append(lines, "  "+normalItemStyle.Render(wl))
			}
		}
	}

	// ── Description (shown last — less critical than PRs and files) ──
	desc := s.Description
	if desc == "" {
		desc = s.Title
	}
	if desc != "" {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("  "+strings.Repeat("─", width-4)))
		lines = append(lines, "")
		lines = append(lines, panelHeaderStyle.Render("  Description"))
		lines = append(lines, "")

		wrapped := wordWrap(desc, wrapWidth)
		for _, wl := range wrapped {
			lines = append(lines, "  "+normalItemStyle.Render(wl))
		}
	}

	return lines
}

// =====================================================================
// RUN + RESUME
// =====================================================================

func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(Model)
	if !ok {
		return nil
	}

	if m.resuming != nil {
		return resumeSession(m.resuming)
	}

	return nil
}

func resumeSession(s *scanner.Session) error {
	fmt.Printf("\nResuming session %s in %s...\n\n", s.ID[:8], s.ProjectPath)

	cmd := exec.Command("claude", "--resume", s.ID)
	cmd.Dir = s.ProjectPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// =====================================================================
// HELPERS
// =====================================================================

// shortenPath returns the last 2 segments of a path, joined with " > "
// instead of "/" to prevent terminals from interpreting it as a URL.
// "/Users/<user>/git/my-project" → "git > my-project"
func shortenPath(path string) string {
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) <= 3 { return path }
	return strings.Join(parts[len(parts)-2:], " > ")
}

// visualWidth calculates the true display width of a styled string.
// lipgloss.Width() correctly handles:
//   - ANSI escape codes (invisible styling characters)
//   - Unicode characters (emoji, CJK, box-drawing chars)
//   - Multi-byte UTF-8 characters
// Our old custom stripAnsi was buggy — lipgloss handles all edge cases.
func visualWidth(s string) int {
	return lipgloss.Width(s)
}

// wordWrap breaks a long string into multiple lines, each at most `width` characters.
// It breaks at word boundaries (spaces) to avoid cutting words in half.
// For long words with no spaces (like URLs), it hard-breaks at `width`.
// makeHyperlink creates an OSC 8 terminal hyperlink.
// The text is rendered as a clickable label — clicking opens the URL in a browser.
// makeHyperlink creates a clickable terminal hyperlink using OSC 8 escape sequence.
func makeHyperlink(url, label string) string {
	return util.MakeHyperlink(url, label)
}

func wordWrap(text string, width int) []string {
	if width <= 0 { return []string{text} }

	words := strings.Fields(text)
	if len(words) == 0 { return nil }

	var lines []string
	currentLine := ""

	for _, word := range words {
		// If the word itself is longer than width, hard-break it.
		// This handles URLs which have no spaces.
		for len(word) > width {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = ""
			}
			lines = append(lines, word[:width])
			word = word[width:]
		}

		if currentLine == "" {
			currentLine = word
		} else if len(currentLine)+1+len(word) > width {
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

