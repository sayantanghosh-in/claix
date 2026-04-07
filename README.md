```
 ██████╗ ██╗       █████╗  ██╗ ██╗  ██╗
██╔════╝ ██║      ██╔══██╗ ██║  ██╗██╔╝
██║      ██║      ███████║ ██║   ████╔╝ 
██║      ██║      ██╔══██║ ██║  ██╔╝██╗ 
╚██████╗ ███████╗ ██║  ██║ ██║ ██╔╝  ██╗
 ╚═════╝ ╚══════╝ ╚═╝  ╚═╝ ╚═╝ ╚═╝  ╚═╝
```

![Go](https://img.shields.io/badge/go-1.26+-00ADD8?style=flat-square&logo=go&logoColor=white)
![License](https://img.shields.io/github/license/sayantanghosh-in/claix?style=flat-square&color=green)
![Build](https://img.shields.io/github/actions/workflow/status/sayantanghosh-in/claix/ci.yml?branch=main&style=flat-square&label=build)
![Release](https://img.shields.io/github/v/release/sayantanghosh-in/claix?style=flat-square&color=purple)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square)

> _Make your Claude sessions click._

**claix** is a smart terminal UI to search, organize, and resume your [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions across all your projects. Never lose a conversation again.

---

## Features

- **Session discovery** — Automatically scans and indexes all Claude Code sessions across projects
- **Fuzzy search** — Find any session by what you were working on, not by cryptic IDs
- **One-key resume** — Select a session, hit Enter, and you're back in the conversation
- **Tags & notes** — Label sessions so you remember what you were doing
- **Activity heatmap** — GitHub-style contribution graph of your Claude Code usage
- **Token stats** — Track estimated usage per session, project, and time period
- **Project grouping** — Sessions organized by project with git branch context
- **Auto-sync** — Claude Code hooks capture session data on exit, zero manual effort
- **Export** — Generate markdown summaries for standups, PRs, or blog posts

---

## Installation

### Homebrew (macOS / Linux)

```bash
brew install sayantanghosh-in/tap/claix
```

### Go install

```bash
go install github.com/sayantanghosh-in/claix@latest
```

### From source

```bash
git clone https://github.com/sayantanghosh-in/claix.git
cd claix
make build
./bin/claix
```

### Download binary

Grab the latest release for your platform from [GitHub Releases](https://github.com/sayantanghosh-in/claix/releases).

---

## Quick Start

```bash
# One-time setup — configures Claude Code hooks for auto-sync
claix install

# Launch the TUI — browse, search, and resume sessions
claix

# Or use CLI commands directly
claix list                    # List all sessions
claix search "auth bug"       # Fuzzy search sessions
claix resume                  # Interactive session picker
claix stats                   # Usage heatmap and stats
claix sync                    # Manually sync session data
```

---

## How It Works

claix reads session data from `~/.claude/projects/` where Claude Code stores conversation history. It builds a local index with your tags, notes, and metadata — **it never modifies Claude's files**.

### Dashboard Header

When you launch `claix`, the top of the TUI shows a dashboard with three lines of aggregate stats:

**Line 1 — Session Summary**
```
53 sessions  ● 3 active  ✓ 36 done  ○ 14 empty  │  10 projects
```
- **53 sessions** — total number of Claude Code sessions found across all projects
- **● 3 active** — sessions where your last message hasn't been responded to (you may want to resume these)
- **✓ 36 done** — sessions where Claude responded last (conversation reached a natural end)
- **○ 14 empty** — sessions that were opened but had no messages (hidden from the list by default)
- **10 projects** — number of unique project directories where sessions were started

**Line 2 — Activity Sparkline & Usage**
```
▁▃▇▅▂▆█▃▁▄▆▂▃▅▇▅▂▁▃▆█▅▃▁▂▄▇  28d  │  11.7k msgs  1.9k tools  │  opus 325.7k  sonnet 2.6k
```
- **Sparkline** (`▁▃▇▅...`) — a 28-day activity chart where each bar represents one day's message count. Taller bars = more active days. Gives you a quick sense of your usage pattern over the last month.
- **28d** — the time range the sparkline covers (28 days)
- **11.7k msgs** — total messages exchanged with Claude across all sessions
- **1.9k tools** — total tool calls Claude made (file reads, edits, bash commands, etc.)
- **opus 325.7k / sonnet 2.6k** — token usage broken down by Claude model. This data comes from Claude Code's internal `stats-cache.json`.

**Line 3 — Top Projects**
```
git > my-project ████████ 13    git > another-project █████ 7    <user> > git ████ 6
```
- Shows your **top 3 most active projects** ranked by number of sessions
- The **bar chart** (█░) is proportional — the project with the most sessions gets a full bar, others are scaled relative to it
- The **number** is the total session count for that project

### Auto-sync with Hooks

After running `claix install`, a Claude Code hook is configured to run `claix sync` whenever a session ends. This means your session index stays up to date without any manual effort.

### Architecture

```
~/.claude/projects/          ← Claude Code's session data (read-only)
~/.config/claix/
  ├── config.json            ← claix configuration
  └── store.json             ← your tags, notes, pins (claix's own data)
```

---

## CLI Commands

```bash
claix                        # Launch TUI (default)
claix list                   # List all sessions across projects
claix list --project <path>  # Filter by project
claix search <query>         # Fuzzy search by content/topic
claix resume                 # Interactive resume picker
claix stats                  # Activity heatmap and usage stats
claix sync                   # Re-index session data
claix install                # Set up hooks and MCP integration
claix version                # Print version
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Resume selected session |
| `/` | Search / filter |
| `t` | Add/edit tags |
| `n` | Add/edit notes |
| `p` | Pin/unpin session |
| `s` | Sort (date, project, tags) |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

---

## Configuration

claix stores its config at `~/.config/claix/config.json`:

```json
{
  "claude_home": "~/.claude",
  "theme": "default",
  "sort_by": "last_active"
}
```

---

## Development

### Prerequisites

- Go 1.22+
- Make

### Setup

```bash
git clone https://github.com/sayantanghosh-in/claix.git
cd claix
go mod tidy
make build
make run
```

### Project Structure

```
claix/
├── main.go                    # Entry point
├── cmd/claix/
│   └── root.go                # Cobra CLI commands
├── internal/
│   ├── tui/
│   │   ├── app.go             # Root Bubbletea model
│   │   ├── styles.go          # Lipgloss styles
│   │   ├── keys.go            # Keybindings
│   │   └── views/
│   │       ├── list.go        # Session list view
│   │       ├── detail.go      # Session detail view
│   │       └── help.go        # Help overlay
│   ├── scanner/
│   │   └── scanner.go         # Claude session file parser
│   ├── config/
│   │   └── config.go          # Configuration management
│   └── store/
│       └── store.go           # Local metadata store
├── docs/                      # Documentation
├── .github/workflows/         # CI/CD
├── .goreleaser.yaml           # Cross-platform release builds
├── Makefile                   # Build automation
└── LICENSE
```

### Running Tests

```bash
make test
```

### Linting

```bash
make lint
```

---

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](docs/CONTRIBUTING.md) for guidelines.

1. Fork the repo
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## Roadmap

- [ ] Core TUI with session list and search
- [ ] Session tagging and notes
- [ ] One-key resume from TUI
- [ ] Auto-sync via Claude Code hooks
- [ ] Activity heatmap and usage stats
- [ ] Token/cost estimation
- [ ] Export session summaries as markdown
- [ ] MCP server for in-session interaction
- [ ] Multi-machine sync

---

## License

[MIT](LICENSE)

---

> _"CLI + AI + X — everything just clicks."_

_Built by [Sayantan Ghosh](https://github.com/sayantanghosh-in)_
