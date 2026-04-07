# VHS tape file for recording the claix demo GIF
# Run: CLAUDE_HOME=~/.claude-demo vhs docs/demo.tape
# Output: docs/demo.gif

Output docs/demo.gif
Set Width 1400
Set Height 800
Set FontSize 14
Set Padding 20
Set TypingSpeed 80ms

# Launch TUI
Hide
Type "clear"
Enter
Sleep 500ms
Show
Type "claix"
Enter
Sleep 2.5s

# Navigate through sessions — let the user see the detail panel update
Down
Sleep 1.5s
Down
Sleep 1.5s
Down
Sleep 1.5s

# Scroll detail panel to show more content
Right
Sleep 600ms
Right
Sleep 600ms
Right
Sleep 1.5s

# Search for "deploy" — shows 2 results (yolo-deploy sessions)
Type "/"
Sleep 1s
Type "deploy"
Sleep 2s

# Clear search
Escape
Sleep 1.5s

# Navigate back to the first session so tag is visible at the top
Up
Sleep 1s
Up
Sleep 1s
Up
Sleep 1.5s

# Add a tag to the selected session
Type "t"
Sleep 1s
Type "important"
Sleep 1.5s
Enter
Sleep 2s

# Add another tag to show it supports multiple
Type "t"
Sleep 1s
Type "needs-review"
Sleep 1.5s
Enter
Sleep 2s

# Scroll detail panel to the top so the Tags section is visible on the right
Left
Sleep 400ms
Left
Sleep 400ms
Left
Sleep 400ms
Left
Sleep 400ms
Left
Sleep 400ms
Left
Sleep 3s

# Remove the "needs-review" tag using x
Type "x"
Sleep 1s
Down
Sleep 800ms
Enter
Sleep 2s

# Quit TUI
Type "q"
Sleep 1.5s

# Show theme switching
Type "claix theme"
Enter
Sleep 2.5s

# Switch to dracula
Type "claix theme dracula"
Enter
Sleep 1.5s

# Show TUI in dracula theme
Type "claix"
Enter
Sleep 2.5s

# Quit
Type "q"
Sleep 1.5s

# Switch back to default
Type "claix theme default"
Enter
Sleep 1.5s
