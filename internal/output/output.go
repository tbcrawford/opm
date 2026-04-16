// Package output provides terminal rendering helpers for opm commands.
// Color is suppressed automatically when stdout is not a terminal (via fatih/color)
// and when the NO_COLOR environment variable is set.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/opm-cli/opm/internal/store"
)

var (
	green  = color.New(color.FgGreen)
	blue   = color.New(color.FgBlue, color.Bold)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	dim    = color.New(color.Faint)
)

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

// ProfileTable writes the ls listing: one profile per line with ●/○/✗ markers.
func ProfileTable(w io.Writer, profiles []store.Profile) {
	for _, p := range profiles {
		switch {
		case p.Dangling:
			fmt.Fprintf(w, "%s %s %s\n", red.Sprint("✗"), red.Sprint(p.Name), dim.Sprint("(missing)"))
		case p.Active:
			fmt.Fprintf(w, "%s %s\n", green.Sprint("●"), blue.Sprint(p.Name))
		default:
			fmt.Fprintf(w, "%s\n", dim.Sprintf("○ %s", p.Name))
		}
	}
}
