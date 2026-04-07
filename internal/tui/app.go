package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sayantanghosh-in/claix/internal/scanner"
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
}

// dataLoadedMsg is sent when session scan + stats load complete.
type dataLoadedMsg struct {
	sessions []scanner.Session
	stats    *scanner.Stats
	err      error
}

func New() Model {
	return Model{loaded: false}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		sessions, err := scanner.ScanAll("")
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		stats, _ := scanner.LoadStats("")
		return dataLoadedMsg{sessions: sessions, stats: stats}
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
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.panelScroll = 0 // Reset panel scroll when switching sessions
			}
		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				m.panelScroll = 0 // Reset panel scroll when switching sessions
			}
		// Scroll the right detail panel up/down with left/right or shift+up/down
		case "right", "l", "shift+down":
			m.panelScroll++
		case "left", "h", "shift+up":
			if m.panelScroll > 0 {
				m.panelScroll--
			}
		case "enter":
			if len(m.sessions) > 0 && m.cursor < len(m.sessions) {
				selected := m.sessions[m.cursor]
				m.resuming = &selected
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	return m, nil
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

	// Calculate column widths: 4:1 ratio
	// Right panel gets 1/5 of width, left gets the rest
	rightWidth := m.width / 5
	if rightWidth < 25 {
		rightWidth = 25
	}
	if rightWidth > 40 {
		rightWidth = 40
	}
	leftWidth := m.width - rightWidth - 3 // -3 for the vertical divider + padding

	// Build left and right columns separately, then merge line by line
	leftLines := m.renderLeftColumn(leftWidth)
	allRightLines := m.renderRightColumn(rightWidth)

	// Apply scroll offset to right panel. Clamp so we don't scroll past the end.
	maxScroll := len(allRightLines) - m.height + 2
	if maxScroll < 0 { maxScroll = 0 }
	if m.panelScroll > maxScroll { m.panelScroll = maxScroll }
	rightLines := allRightLines
	if m.panelScroll > 0 && m.panelScroll < len(allRightLines) {
		rightLines = allRightLines[m.panelScroll:]
	}

	// Merge left and right columns line by line.
	// Each left line is padded to exactly leftWidth so the divider aligns perfectly.
	var b strings.Builder

	// Use terminal height as the line count — fill the full screen
	totalLines := m.height
	if totalLines < 1 { totalLines = len(leftLines) }

	divider := dimStyle.Render("│")

	for i := 0; i < totalLines; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}

		// Use lipgloss.Width for accurate visual width calculation.
		// This correctly handles ANSI codes, Unicode, and multi-byte chars.
		vw := lipgloss.Width(left)
		pad := leftWidth - vw
		if pad < 0 {
			// Left line is too wide — truncate it (shouldn't happen, but safety)
			pad = 0
		}

		b.WriteString(left)
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(" " + divider + " ")
		b.WriteString(right)
		b.WriteString("\n")
	}

	return b.String()
}

// =====================================================================
// LEFT COLUMN — Header + Session Cards
// =====================================================================

func (m Model) renderLeftColumn(width int) []string {
	var lines []string

	// ── Title ──
	title := titleStyle.Render(" CLAIX ")
	tagline := taglineStyle.Render(" make your Claude sessions click")
	lines = append(lines, title+tagline)

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

	// ── Session cards ──
	// Each card = 3 lines + 1 blank = 4 lines
	headerLineCount := len(lines)
	footerLines := 3
	availableLines := m.height - headerLineCount - footerLines
	linesPerCard := 4
	visibleCards := availableLines / linesPerCard
	if visibleCards < 2 { visibleCards = 2 }

	start := 0
	if m.cursor >= visibleCards {
		start = m.cursor - visibleCards + 1
	}
	end := start + visibleCards
	if end > len(m.sessions) { end = len(m.sessions) }

	for i := start; i < end; i++ {
		s := m.sessions[i]
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

		branch := s.GitBranch
		if branch == "" || branch == "HEAD" {
			branch = ""
		} else {
			if len(branch) > 18 { branch = branch[:18] + ".." }
			branch = "  " + branchStyle.Render(branch)
		}

		// Date + time
		lastActive := s.LastActive
		if len(lastActive) >= 16 {
			lastActive = strings.Replace(lastActive[:16], "T", " ", 1)
		}

		// Line 1: status + id + project + branch ... datetime
		// Build the full styled line, then use lipgloss.Width() to calculate
		// the exact padding needed to right-align the datetime.

		// Truncate branch for display
		branchTrunc := s.GitBranch
		if len(branchTrunc) > 18 { branchTrunc = branchTrunc[:18] + ".." }

		var fullLine string
		if isSelected {
			styledContent := fmt.Sprintf("%s %s  %s%s",
				statusIcon,
				accentStyle.Render(shortID),
				selectedItemStyle.Render(project),
				func() string { if branch != "" { return "  " + branchStyle.Render(branchTrunc) }; return "" }(),
			)
			fullLine = fmt.Sprintf(" %s %s %s",
				selectedCursor.Render("▸"), border, styledContent)
		} else {
			styledContent := fmt.Sprintf("%s %s  %s%s",
				statusIcon,
				dimStyle.Render(shortID),
				project,
				func() string { if branch != "" { return "  " + branchStyle.Render(branchTrunc) }; return "" }(),
			)
			fullLine = fmt.Sprintf("   %s %s", border, styledContent)
		}

		// Right-align datetime: total line width should equal `width`
		styledTime := dimStyle.Render(lastActive)
		usedWidth := lipgloss.Width(fullLine) + lipgloss.Width(styledTime)
		pad := width - usedWidth
		if pad < 2 { pad = 2 }

		lines = append(lines, fullLine + strings.Repeat(" ", pad) + styledTime)

		// Line 2: title — if the session has PRs, make the PR number a clickable hyperlink
		titleText := s.Title
		maxW := width - 12
		if maxW < 20 { maxW = 20 }
		if len(titleText) > maxW { titleText = titleText[:maxW] + "..." }

		// If this session has PRs, replace "PR #N" in the title with a clickable OSC 8 hyperlink
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
			// Extract repo name and append after the hyperlink
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
			lines = append(lines, fmt.Sprintf("     %s   %s", border, titleRendered))
		} else {
			lines = append(lines, fmt.Sprintf("   %s   %s", border, titleRendered))
		}

		// Line 3: file activity summary (compact)
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
			lines = append(lines, fmt.Sprintf("   %s   %s", border, dimStyle.Render(actLine)))
		} else {
			lines = append(lines, fmt.Sprintf("   %s", border))
		}

		// Blank line between cards
		if i < end-1 { lines = append(lines, "") }
	}

	// ── Footer ──
	lines = append(lines, "")
	if len(m.sessions) > visibleCards {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  %d-%d of %d sessions (empty hidden)", start+1, end, len(m.sessions))))
	}
	lines = append(lines, fmt.Sprintf("  %s %s  %s %s  %s %s  %s %s",
		helpKeyStyle.Render("↑↓"), helpDescStyle.Render("navigate"),
		helpKeyStyle.Render("←→"), helpDescStyle.Render("scroll panel"),
		helpKeyStyle.Render("enter"), helpDescStyle.Render("resume"),
		helpKeyStyle.Render("q"), helpDescStyle.Render("quit"),
	))

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

	// ── Session ID ──
	shortID := s.ID
	if len(shortID) > 8 { shortID = shortID[:8] }
	lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("ID:"), accentStyle.Render(shortID)))

	// ── Project ──
	lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Project:"), shortenPath(s.ProjectPath)))

	// ── Branch ──
	if s.GitBranch != "" && s.GitBranch != "HEAD" {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Branch:"), branchStyle.Render(s.GitBranch)))
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

	// ── Slug (Claude's fun session name) ──
	if s.Slug != "" {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("Slug:"), s.Slug))
	}

	// ── Version ──
	if s.Version != "" {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("CLI:"), s.Version))
	}

	// Wrap width for text content in the panel
	wrapWidth := width - 4
	if wrapWidth < 15 { wrapWidth = 15 }

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
				lines = append(lines, "   "+normalItemStyle.Render(repo))
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
// Format: \033]8;;URL\033\\LABEL\033]8;;\033\\
// Supported by: iTerm2, Warp, Ghostty, VS Code terminal, Kitty, WezTerm.
func makeHyperlink(url, label string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, label)
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

