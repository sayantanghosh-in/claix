# Architecture

## Overview

claix is built with a clean separation between the CLI layer, TUI layer, and data layer.

```
┌─────────────────────────────────────────┐
│              CLI (Cobra)                │
│  claix list | search | resume | stats   │
├─────────────────────────────────────────┤
│            TUI (Bubbletea)              │
│  ┌──────┐  ┌────────┐  ┌──────┐        │
│  │ List │  │ Detail │  │ Help │        │
│  └──────┘  └────────┘  └──────┘        │
├─────────────────────────────────────────┤
│           Data Layer                    │
│  ┌─────────┐  ┌─────────┐  ┌────────┐  │
│  │ Scanner │  │  Store  │  │ Config │  │
│  └─────────┘  └─────────┘  └────────┘  │
├─────────────────────────────────────────┤
│           External                      │
│  ~/.claude/projects/    (read-only)     │
│  ~/.config/claix/       (read-write)    │
└─────────────────────────────────────────┘
```

## Packages

### `cmd/claix`
CLI entry point using [Cobra](https://github.com/spf13/cobra). Defines all subcommands and flags. Each command either prints output directly or launches the TUI.

### `internal/tui`
The terminal UI built with [Bubbletea](https://github.com/charmbracelet/bubbletea). Follows the Elm architecture (Model → Update → View). The root model (`app.go`) multiplexes between child views using message-based navigation.

**Key dependencies:**
- `bubbletea` — Elm-style TUI framework
- `bubbles` — Pre-built components (list, text input, help, spinner)
- `lipgloss` — Styling and layout

### `internal/scanner`
Reads and parses Claude Code session files from `~/.claude/projects/`. This package only reads — it never writes to Claude's directories. It extracts session metadata: ID, project path, timestamps, conversation content for summaries.

### `internal/store`
Manages claix's own metadata — tags, notes, pinned status — stored in `~/.config/claix/store.json`. This is merged with scanner data at display time so users see a unified view.

### `internal/config`
Configuration management. Loads settings from `~/.config/claix/config.json` with sensible defaults. Supports XDG base directories.

## Data Flow

1. **Scanner** reads raw session data from Claude's files
2. **Store** loads user metadata (tags, notes)
3. Data is merged and passed to the **TUI** or **CLI** for display
4. User actions (tagging, noting) are written back to the **Store** only

## Design Principles

- **Read-only access to Claude's data** — We never modify `~/.claude/`
- **Graceful degradation** — Missing or corrupt files are skipped, not fatal
- **Fast startup** — Index lazily, cache aggressively
- **Single binary** — No external dependencies at runtime
