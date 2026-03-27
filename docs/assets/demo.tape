# csaw demo recording
# Re-record: vhs docs/assets/demo.tape

Output docs/assets/demo.gif
Set Theme "Catppuccin Mocha"
Set FontSize 16
Set Width 900
Set Height 500
Set Padding 20

Type "# Add a registry source"
Enter
Sleep 500ms
Type "csaw source add dotagent ~/Developer/dotagent"
Enter
Sleep 1500ms

Type "csaw source list"
Enter
Sleep 1500ms

Enter
Type "# Mount a profile — skills go into tool directories"
Enter
Sleep 500ms
Type "mkdir -p .claude && csaw mount --profile dotagent/go --force"
Enter
Sleep 2000ms

Enter
Type "# Inspect what's active"
Enter
Sleep 500ms
Type "csaw inspect"
Enter
Sleep 3000ms

Enter
Type "# Check health of mounted links"
Enter
Sleep 500ms
Type "csaw check"
Enter
Sleep 2000ms

Enter
Type "# Clean unmount — no trace left behind"
Enter
Sleep 500ms
Type "csaw unmount"
Enter
Sleep 1500ms

Type "csaw inspect"
Enter
Sleep 2000ms

Sleep 1000ms
