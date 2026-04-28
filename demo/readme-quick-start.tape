Output "assets/demo/readme-quick-start.gif"

Require just
Require go
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
Type "just assemble"
Enter
Type "export DEMO_HOME=$(mktemp -d)"
Enter
Type "export HOME=$DEMO_HOME"
Enter
Type "export PATH=$PWD:$PATH"
Enter
Type "mkdir -p $HOME/.config/opencode"
Enter
Type "echo Demo profile > $HOME/.config/opencode/AGENTS.md"
Enter
Type "clear"
Enter
Show

Type "opm init"
Enter
Sleep 1.2s

Type "opm create work"
Enter
Sleep 1.0s

Type "opm use work"
Enter
Sleep 1.0s

Type "opm list"
Enter
Sleep 1.5s

Type "opm create personal"
Enter
Sleep 0.8s

# Show exec: spawn a command under the personal profile — global profile unchanged.
Type "opm exec personal -- sh -c 'basename $(readlink $XDG_CONFIG_HOME/opencode)'"
Enter
Sleep 1.5s

# The global active profile is still "work".
Type "opm show"
Enter
Sleep 1.5s

Hide
Type "rm -f opm"
Enter
Type "rm -rf $HOME"
Enter
