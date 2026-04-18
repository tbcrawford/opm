package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
)

var copyCmd = &cobra.Command{
	Use:               "copy <src> <dst>",
	Short:             "Copy a profile to a new name",
	Args:              cobra.ExactArgs(2),
	PreRunE:           managedGuard,
	ValidArgsFunction: singleArgProfileCompletion,
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
	output.Success(cmd.OutOrStdout(),
		"Copied "+output.ProfileName(src)+" → "+output.ProfileName(dst),
		output.ShortenHome(s.ProfileDir(dst))+"/",
	)
	return nil
}
