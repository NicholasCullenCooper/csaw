# csaw demo recording
# Re-record: vhs docs/assets/demo.tape

Output docs/assets/demo.gif
Set Theme "Catppuccin Mocha"
Set FontSize 15
Set Width 900
Set Height 550
Set Padding 20
Set TypingSpeed 35ms

# Add a source
Type "csaw source add team git@github.com:your-org/ai-config.git"
Enter
Sleep 2000ms

# Mount a profile
Enter
Type "csaw mount --profile team/backend"
Enter
Sleep 2500ms

# Inspect
Enter
Type "csaw inspect"
Enter
Sleep 3000ms

# Check health
Enter
Type "csaw check"
Enter
Sleep 1200ms

# Clean unmount
Enter
Type "csaw unmount"
Enter
Sleep 1500ms

Sleep 800ms
