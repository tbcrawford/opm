package cmd

import (
	"bytes"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
)

const rootHelpGroupAnnotation = "opm/root-help-group"
const rootHelpOrderAnnotation = "opm/root-help-order"

const (
	helpGroupSetup     = "Setup"
	helpGroupProfiles  = "Profiles"
	helpGroupScripting = "Scripting"
)

var rootHelpGroupOrder = []string{
	helpGroupSetup,
	helpGroupProfiles,
	helpGroupScripting,
}

var rootHelpGroups = map[string]bool{
	helpGroupSetup:     true,
	helpGroupProfiles:  true,
	helpGroupScripting: true,
}

type rootHelpEntry struct {
	name  string
	short string
	alias string
}

type rootHelpSection struct {
	label   string
	entries []rootHelpEntry
}

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
	sections := buildRootHelpSections(root)

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

func markRootHelpGroup(cmd *cobra.Command, group string) {
	if !rootHelpGroups[group] {
		panic("unknown root help group: " + group)
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[rootHelpGroupAnnotation] = group
}

func markRootHelpOrder(cmd *cobra.Command, order int) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[rootHelpOrderAnnotation] = fmt.Sprintf("%03d", order)
}

func buildRootHelpSections(root *cobra.Command) []rootHelpSection {
	type orderedEntry struct {
		entry rootHelpEntry
		order string
	}
	grouped := make(map[string][]orderedEntry)
	for _, cmd := range root.Commands() {
		group := cmd.Annotations[rootHelpGroupAnnotation]
		if group == "" || cmd.Hidden {
			continue
		}
		if !rootHelpGroups[group] {
			panic("unknown root help group: " + group)
		}
		entry := rootHelpEntry{
			name:  cmd.Name(),
			short: cmd.Short,
		}
		if len(cmd.Aliases) > 0 {
			entry.alias = cmd.Aliases[0]
		}
		grouped[group] = append(grouped[group], orderedEntry{entry: entry, order: cmd.Annotations[rootHelpOrderAnnotation]})
	}

	var sections []rootHelpSection
	for _, group := range rootHelpGroupOrder {
		entries := grouped[group]
		if len(entries) == 0 {
			continue
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].order == entries[j].order {
				return entries[i].entry.name < entries[j].entry.name
			}
			return entries[i].order < entries[j].order
		})
		section := rootHelpSection{label: group, entries: make([]rootHelpEntry, 0, len(entries))}
		for _, entry := range entries {
			section.entries = append(section.entries, entry.entry)
		}
		sections = append(sections, section)
	}
	return sections
}
