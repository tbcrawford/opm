package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	green  = color.New(color.FgGreen)
	blue   = color.New(color.FgBlue, color.Bold)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	dim    = color.New(color.Faint)
)

func init() {
	// Disable color when stdout is not a terminal (e.g. piped to grep/file).
	// fatih/color additionally respects the NO_COLOR env var automatically.
	if fi, err := os.Stdout.Stat(); err == nil {
		if fi.Mode()&os.ModeCharDevice == 0 {
			color.NoColor = true
		}
	}
}

// Success prints a green ✓ line followed by optional dim detail lines.
// Used by all state-changing commands on success.
func Success(w io.Writer, msg string, detail ...string) {
	fmt.Fprintf(w, "%s %s\n", green.Sprint("✓"), msg)
	for _, d := range detail {
		fmt.Fprintf(w, "%s\n", dim.Sprintf("  %s", d))
	}
}

// Failure prints a red ✗ line followed by optional dim detail lines.
// Used for non-fatal in-command failure messages printed before returning an error.
func Failure(w io.Writer, msg string, detail ...string) {
	fmt.Fprintf(w, "%s %s\n", red.Sprint("✗"), msg)
	for _, d := range detail {
		fmt.Fprintf(w, "%s\n", dim.Sprintf("  %s", d))
	}
}

// Error prints a red ✗ first line, then remaining lines with dim indent preserved.
// Used by Execute() to format all command errors uniformly.
func Error(w io.Writer, msg string) {
	parts := strings.SplitN(msg, "\n", 2)
	fmt.Fprintf(w, "%s %s\n", red.Sprint("✗"), parts[0])
	if len(parts) > 1 && parts[1] != "" {
		fmt.Fprintln(w, dim.Sprint(parts[1]))
	}
}

// ProfileName returns the profile name formatted as bold blue for inline use in strings.
func ProfileName(name string) string {
	return blue.Sprint(name)
}

// ShortenHome replaces the user's home directory prefix with ~.
func ShortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
