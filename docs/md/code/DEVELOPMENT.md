# Development Guide

## Quick Reference

```bash
make build    # Build binary to ./bin/claix
make run      # Build and run
make test     # Run tests with race detector
make lint     # Run golangci-lint
make clean    # Remove build artifacts
```

## Project Layout

```
main.go              → Entry point, calls cmd/claix.Execute()
cmd/claix/root.go    → All CLI commands (Cobra)
internal/tui/        → Bubbletea TUI (app, views, styles, keys)
internal/scanner/    → Claude session file reader
internal/store/      → claix metadata persistence (tags, notes)
internal/config/     → Configuration loading
```

## Adding a New CLI Command

1. Add a `*cobra.Command` in `cmd/claix/root.go`
2. Register it in `init()` with `rootCmd.AddCommand(yourCmd)`
3. Implement the handler — either print output or launch TUI

## Adding a New TUI View

1. Create a new file in `internal/tui/views/`
2. Implement `Init()`, `Update()`, `View()` on your model
3. Define navigation messages (e.g., `NavigateToYourViewMsg`)
4. Handle the message in `internal/tui/app.go` to switch views

## Styling

All styles live in `internal/tui/styles.go`. Use lipgloss — never hardcode ANSI escape sequences.

## Testing

```bash
# All tests
make test

# Specific package
go test ./internal/scanner/... -v

# With coverage
go test ./... -coverprofile=coverage.txt
go tool cover -html=coverage.txt
```

## Releasing

Releases are automated via GoReleaser + GitHub Actions. To create a release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This triggers the release workflow which builds binaries for all platforms and publishes to GitHub Releases.
