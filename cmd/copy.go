package cmd

import (
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/paths"
	"github.com/spf13/cobra"
)

var copyCmd = &cobra.Command{
	Use:               "copy <src> <dst>",
	Short:             "Copy a profile to a new name",
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runCopy,
}

func init() {
	rootCmd.AddCommand(copyCmd)
}

func runCopy(cmd *cobra.Command, args []string) error {
	src, dst := args[0], args[1]
	s := newStore()
	if err := s.CopyProfile(src, dst); err != nil {
		return err
	}
	dstDir := paths.ProfileDir(dst)
	output.Success(cmd.OutOrStdout(),
		"Copied "+output.ProfileName(src)+" → "+output.ProfileName(dst),
		output.ShortenHome(dstDir)+"/",
	)
	return nil
}
