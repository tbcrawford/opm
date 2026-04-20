Output "assets/demo/readme-quick-start.gif"

Require just
Require mkdir
Require rm
Require env

Set Shell "zsh"
Set FontSize 22
Set Width 920
Set Height 640
Set Padding 24
Set Margin 12
Set BorderRadius 8
Set Framerate 30
Set PlaybackSpeed 1.0
Set CursorBlink false
Set Theme "GitHub Dark"
Set WindowBar Colorful
Set TypingSpeed 90ms

# Regenerate with: just demo-readme

Hide
Type "export DEMO_HOME=$(mktemp -d)"
Enter
Type "mkdir -p $DEMO_HOME/.config/opencode"
Enter
Type "echo Demo profile > $DEMO_HOME/.config/opencode/AGENTS.md"
Enter
Type "clear"
Enter
Show

Type "HOME=$DEMO_HOME just run init"
Enter
Sleep 1.2s

Type "HOME=$DEMO_HOME just run create work"
Enter
Sleep 1.0s

Type "HOME=$DEMO_HOME just run use work"
Enter
Sleep 1.0s

Type "HOME=$DEMO_HOME just run list"
Enter
Sleep 1.5s

Hide
Type "rm -rf $DEMO_HOME"
Enter
