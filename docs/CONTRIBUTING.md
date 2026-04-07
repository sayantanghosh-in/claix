# Contributing to claix

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Make](https://www.gnu.org/software/make/)
- [golangci-lint](https://golangci-lint.run/welcome/install/) (for linting)

### Getting Started

```bash
# Fork and clone
git clone https://github.com/<your-username>/claix.git
cd claix

# Install dependencies
go mod tidy

# Build
make build

# Run
make run

# Test
make test

# Lint
make lint
```

## Code Guidelines

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep packages focused — one responsibility per package
- Write table-driven tests where possible
- Use `lipgloss` for all TUI styling (no hardcoded ANSI codes)
- Never write to `~/.claude/` — Claude's data is read-only

## Commit Messages

Use conventional commit style:

```
feat: add session tagging
fix: handle missing session files gracefully
docs: update keyboard shortcuts table
refactor: extract session parsing into scanner package
test: add scanner unit tests
```

## Pull Requests

1. Create a feature branch from `main`
2. Keep PRs focused — one feature or fix per PR
3. Include tests for new functionality
4. Update docs if you change user-facing behavior
5. Ensure CI passes before requesting review

## Reporting Issues

- Use GitHub Issues
- Include your OS, Go version, and claix version
- Steps to reproduce are always helpful

## Project Structure

See [ARCHITECTURE.md](ARCHITECTURE.md) for a full overview of the codebase.
