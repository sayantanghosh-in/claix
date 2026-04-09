# VHS tape file for recording the claix TUI demo GIF
# Run from project root: vhs docs/tapes/demo.tape

Output docs/images/gif/demo.gif
Set Width 1400
Set Height 800
Set FontSize 14
Set Padding 20
Set TypingSpeed 80ms
Set Shell "zsh"

# Setup env invisibly
Hide
Type "export CLAUDE_HOME=$HOME/.claude-demo"
Enter
Type "clear"
Enter
Sleep 500ms
Show

# Show "claix" being typed
Type "claix"
Sleep 300ms

# Hide, swap to go run ., wait for TUI to load, then show
Hide
Ctrl+U
Type "go run ."
Enter
Sleep 3s
Show

# Navigate down through sessions
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1s

# Search for "deploy"
Type "/"
Sleep 500ms
Type "deploy"
Sleep 2s

# Clear search
Escape
Sleep 1s

# Navigate back up
Up
Sleep 800ms
Up
Sleep 800ms
Up
Sleep 800ms

# Tag the selected session
Type "t"
Sleep 500ms
Type "important"
Enter
Sleep 2s

# Done
