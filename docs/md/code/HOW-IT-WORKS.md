# How It Works

A technical deep dive into how claix discovers, indexes, and displays your Claude Code sessions.

## Table of Contents

- [Data Flow](#data-flow)
- [What claix Reads from Sessions](#what-claix-reads-from-sessions)
- [Dashboard Header Explained](#dashboard-header-explained)

---

## Data Flow

```
~/.claude/projects/           <- Claude Code's session files (read-only)
        |
        v
   claix scanner              <- Reads .jsonl files, extracts metadata
        |
        v
~/.config/claix/
  ├── index.json              <- Cached session index (from claix sync)
  └── store.json              <- Your tags, notes, pins, theme, pending inits
        |
        v
   claix TUI / CLI            <- Displays everything, never modifies Claude's files
```

claix is strictly read-only with respect to Claude Code's data. It reads `.jsonl` session files from `~/.claude/projects/`, extracts metadata, and caches the results in its own index. All user data (tags, notes, theme preferences) is stored separately in `~/.config/claix/store.json`.

---

## What claix Reads from Sessions

Each Claude Code session is a `.jsonl` file. claix extracts the following:

| Data | Source | Used for |
|------|--------|----------|
| Session ID | Filename | Identification, resume command |
| Timestamps | `timestamp` field | Sorting, "last active" display |
| Git branch | `user.gitBranch` | Branch display, search |
| Project path | Parent directory name | Project grouping |
| File paths | `tool_use` blocks (Read/Edit) | "Files touched" summary |
| PR links | `pr-link` entries | Clickable PR links |
| Messages | `user`/`assistant` entries | Auto-titles, previews, export |
| Session slug | `assistant.slug` | Fun session name display |

---

## Dashboard Header Explained

The dashboard header at the top of the TUI displays three lines of summary information.

### Line 1 — Session Summary

```
53 sessions  ● 3 active  ✓ 36 done  ○ 14 empty  │  10 projects
```

- **● active** — your last message hasn't been responded to (you may want to resume)
- **✓ done** — Claude responded last (conversation ended naturally)
- **○ empty** — opened but no messages exchanged (hidden by default)

### Line 2 — Activity Sparkline & Usage

```
▁▃▇▅▂▆█▃▁▄▆▂▃▅▇▅▂▁▃▆█▅▃▁▂▄▇  28d  │  11.7k msgs  1.9k tools  │  opus 325k  sonnet 2.6k
```

- **Sparkline** — 28-day activity chart, each bar = one day's message count
- **msgs** — total messages exchanged with Claude
- **tools** — total tool calls (file reads, edits, bash commands)
- **opus/sonnet** — token usage by model (from Claude Code's `stats-cache.json`)

### Line 3 — Top Projects

```
git > my-project ████████ 13    git > another-project █████ 7
```

- Top 3 projects by session count with proportional bar charts
