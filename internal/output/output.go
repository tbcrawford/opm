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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	green  = color.New(color.FgGreen)
	blue   = color.New(color.FgBlue, color.Bold)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	dim    = color.New(color.Faint)
)

var (
	steelBlue = color.New(color.FgHiBlue, color.Bold)
	cmdColor  = color.New(color.FgHiWhite, color.Bold)
	flagColor = color.New(color.FgCyan)
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

// ProfileTableLong writes the list with an extra path column, aligned by
// padding the name to the longest name in the list.
func ProfileTableLong(w io.Writer, profiles []store.Profile) {
	// Find the longest name for manual padding (tabwriter cannot account for
	// ANSI escape bytes added by color functions).
	maxLen := 0
	for _, p := range profiles {
		if len(p.Name) > maxLen {
			maxLen = len(p.Name)
		}
	}
	for _, p := range profiles {
		pad := strings.Repeat(" ", maxLen-len(p.Name))
		switch {
		case p.Dangling:
			fmt.Fprintf(w, "%s %s%s    %s\n", red.Sprint("✗"), red.Sprint(p.Name), pad, dim.Sprint("(missing) "+p.Path))
		case p.Active:
			fmt.Fprintf(w, "%s %s%s    %s\n", green.Sprint("●"), blue.Sprint(p.Name), pad, dim.Sprint(ShortenHome(p.Path)))
		default:
			fmt.Fprintf(w, "%s %s%s    %s\n", dim.Sprint("○"), dim.Sprint(p.Name), pad, dim.Sprint(ShortenHome(p.Path)))
		}
	}
}

// InspectProfile writes the inspect block: name header, path row, contents list.
func InspectProfile(w io.Writer, name, path string, active bool, entries []os.DirEntry) {
	// Header: name + optional active badge
	if active {
		fmt.Fprintf(w, "%s %s\n", blue.Sprint(name), green.Sprint("● active"))
	} else {
		fmt.Fprintln(w, blue.Sprint(name))
	}
	fmt.Fprintln(w)

	// Path row
	fmt.Fprintf(w, "%s%s\n", dim.Sprintf("%-9s", "Path"), ShortenHome(path))

	// Contents rows
	if len(entries) == 0 {
		fmt.Fprintf(w, "%s%s\n", dim.Sprintf("%-9s", "Contents"), dim.Sprint("(empty)"))
		return
	}
	for i, e := range entries {
		entryName := e.Name()
		if e.IsDir() {
			entryName += "/"
		}
		label := ""
		if i == 0 {
			label = "Contents"
		}
		fmt.Fprintf(w, "%s%s\n", dim.Sprintf("%-9s", label), entryName)
	}
}

// DoctorStatus represents the result of a single doctor check.
type DoctorStatus int

const (
	StatusOK   DoctorStatus = iota
	StatusWarn              // yellow ⚠
	StatusFail              // red ✗
)

// DoctorRow writes a single doctor check line.
func DoctorRow(w io.Writer, status DoctorStatus, msg string) {
	switch status {
	case StatusOK:
		fmt.Fprintf(w, "  %s  %s\n", green.Sprint("✓"), msg)
	case StatusWarn:
		fmt.Fprintf(w, "  %s  %s\n", yellow.Sprint("⚠"), msg)
	case StatusFail:
		fmt.Fprintf(w, "  %s  %s\n", red.Sprint("✗"), msg)
	default:
		fmt.Fprintf(w, "  %s  %s\n", dim.Sprint("?"), msg)
	}
}

// DoctorSection prints a dim section label for grouping doctor checks.
func DoctorSection(w io.Writer, label string) {
	fmt.Fprintln(w, dim.Sprint(label))
}

// DoctorSummary writes the final summary line for `opm doctor`.
func DoctorSummary(w io.Writer, warnings, failures int) {
	switch {
	case failures == 0 && warnings == 0:
		fmt.Fprintln(w, green.Sprint("✓ All checks passed"))
	case failures == 0:
		fmt.Fprintln(w, yellow.Sprintf("⚠ %d warning(s)", warnings))
	default:
		// Failures dominate; warnings are subsumed into the failure count display.
		fmt.Fprintln(w, red.Sprintf("✗ %d problem(s) found", failures))
	}
}

// HelpHeader writes the top-level header block for `opm --help`.
func HelpHeader(w io.Writer, name, short string) {
	fmt.Fprintf(w, "%s — %s\n\nUsage:\n  %s <command> [flags]\n\n", cmdColor.Sprint(name), short, name)
}

// HelpSection writes a colored section header (e.g. "Setup").
func HelpSection(w io.Writer, label string) {
	fmt.Fprintln(w, steelBlue.Sprint(label))
}

// HelpCommand writes a single command row inside a section.
// alias is optional; pass "" to omit.
// The name and description are tab-separated so callers can pipe through tabwriter.
func HelpCommand(w io.Writer, name, description, alias string) {
	aliasStr := ""
	if alias != "" {
		aliasStr = "  " + dim.Sprintf("(%s)", alias)
	}
	fmt.Fprintf(w, "  %s\t%s%s\n", cmdColor.Sprint(name), description, aliasStr)
}

// HelpFlag returns a formatted flag entry string for inline use.
func HelpFlag(flag, description string) string {
	return fmt.Sprintf("  %s    %s", flagColor.Sprint(flag), description)
}

// HelpFlagTable writes a tab-aligned flag table to w.
// Each entry is [2]string{flagName, description}.
func HelpFlagTable(w io.Writer, flags [][2]string) {
	maxLen := 0
	for _, f := range flags {
		if len(f[0]) > maxLen {
			maxLen = len(f[0])
		}
	}
	for _, f := range flags {
		pad := strings.Repeat(" ", maxLen-len(f[0]))
		fmt.Fprintf(w, "  %s%s    %s\n", flagColor.Sprint(f[0]), pad, f[1])
	}
}

// SubcmdHelp renders a styled help page for a single subcommand.
func SubcmdHelp(w io.Writer, cmd *cobra.Command) {
	useParts := strings.Fields(cmd.Use)
	cmdName := "opm"
	if len(useParts) > 0 {
		cmdName = "opm " + useParts[0]
	}

	fmt.Fprintf(w, "%s — %s\n", steelBlue.Sprint(cmdName), cmd.Short)

	if cmd.Long != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, cmd.Long)
	}

	fmt.Fprintln(w)
	HelpSection(w, "Usage:")
	// Build the full usage line: "opm <use-field>" rather than cmd.UseLine()
	// which omits the root command name for standalone (parentless) commands.
	fmt.Fprintf(w, "  opm %s [flags]\n", cmd.Use)

	allFlags := &pflag.FlagSet{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			allFlags.AddFlag(f)
		}
	})
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden && allFlags.Lookup(f.Name) == nil {
			allFlags.AddFlag(f)
		}
	})

	if allFlags.HasFlags() {
		fmt.Fprintln(w)
		HelpSection(w, "Flags:")
		type flagRow struct {
			name  string
			usage string
		}
		var rows []flagRow
		allFlags.VisitAll(func(f *pflag.Flag) {
			var name string
			if f.Shorthand != "" {
				name = "-" + f.Shorthand + ", --" + f.Name
			} else {
				name = "--" + f.Name
			}
			rows = append(rows, flagRow{name, f.Usage})
		})
		maxLen := 0
		for _, r := range rows {
			if len(r.name) > maxLen {
				maxLen = len(r.name)
			}
		}
		for _, r := range rows {
			pad := strings.Repeat(" ", maxLen-len(r.name))
			fmt.Fprintf(w, "  %s%s    %s\n", flagColor.Sprint(r.name), pad, r.usage)
		}
	}
}
