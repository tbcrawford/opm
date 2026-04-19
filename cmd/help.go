package cmd

import (
	"bytes"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
)

// registerHelp sets a custom help function on the root command.
// Call this from an init() in root.go.
func registerHelp(root *cobra.Command) {
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != root {
			output.SubcmdHelp(cmd.OutOrStdout(), cmd)
			return
		}
		printRootHelp(cmd)
	})
}

// printRootHelp renders the grouped help page for `opm --help`.
func printRootHelp(root *cobra.Command) {
	w := root.OutOrStdout()

	output.HelpHeader(w, "opm", "OpenCode profile manager")

	type entry struct {
		name  string
		short string
		alias string
	}
	type section struct {
		label   string
		entries []entry
	}

	sections := []section{
		{
			label: "Setup",
			entries: []entry{
				{"init", "Initialize opm and migrate your existing OpenCode config", ""},
				{"doctor", "Check opm installation health", ""},
				{"reset", "Remove opm management and restore your config directory", ""},
			},
		},
		{
			label: "Profiles",
			entries: []entry{
				{"create", "Create a new profile", ""},
				{"copy", "Copy an existing profile to a new name", ""},
				{"use", "Switch to a profile", ""},
				{"list", "List all profiles", "ls"},
				{"show", "Show the active profile name", ""},
				{"inspect", "Show profile details and contents", ""},
				{"rename", "Rename a profile", ""},
				{"remove", "Remove one or more profiles", "rm"},
			},
		},
		{
			label: "Scripting",
			entries: []entry{
				{"path", "Print the absolute path to a profile directory", ""},
			},
		},
	}

	for i, sec := range sections {
		output.HelpSection(w, sec.label)
		var buf bytes.Buffer
		tw := tabwriter.NewWriter(&buf, 0, 0, 4, ' ', 0)
		for _, e := range sec.entries {
			output.HelpCommand(tw, e.name, e.short, e.alias)
		}
		_ = tw.Flush()
		_, _ = fmt.Fprint(w, buf.String())
		if i < len(sections)-1 {
			_, _ = fmt.Fprintln(w)
		}
	}

	_, _ = fmt.Fprintln(w)
	output.HelpSection(w, "Flags:")
	output.HelpFlagTable(w, [][2]string{
		{"--version", "Print version and exit"},
		{"--help", "Show this help"},
	})
}
