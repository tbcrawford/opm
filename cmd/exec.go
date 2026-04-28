package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <profile> [-- command [args...]]",
	Short: "Run a command with a specific profile active",
	Long: `Run opencode (or any command) using the named profile, without changing
the global active profile. The spawned process reads its opencode
configuration from the given profile directory.

If no command is provided, opencode is launched.

Examples:
  opm exec work
  opm exec personal -- opencode --no-auto-update
  opm exec ci -- opencode run "fix the tests"`,
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: execCompletion,
	SilenceUsage:      true,
	RunE:              runExec,
}

func init() {
	markRootHelpGroup(execCmd, helpGroupProfiles)
	markRootHelpOrder(execCmd, 35)
	rootCmd.AddCommand(execCmd)
}

// execCompletion completes the first argument (profile name) and offers no
// completions for subsequent arguments (those are passed to the child command).
func execCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		// After the profile name, defer to the shell's own completion.
		return nil, cobra.ShellCompDirectiveDefault
	}
	return completeProfileNames(args, 1, false)
}

func runExec(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	// Remaining args (after optional "--") are the command + arguments to run.
	// If none supplied, default to opencode.
	childArgs := args[1:]
	if len(childArgs) == 0 {
		childArgs = []string{"opencode"}
	}

	s := newStore()
	profileDir, err := s.GetProfile(profileName)
	if err != nil {
		return err
	}

	// Build an ephemeral XDG config root:
	//   <tmpdir>/opencode -> profileDir   (symlink)
	// Setting XDG_CONFIG_HOME=<tmpdir> makes opencode read from profileDir.
	tmpDir, err := os.MkdirTemp("", "opm-ephemeral-*")
	if err != nil {
		return fmt.Errorf("create ephemeral environment: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	opencodeLink := filepath.Join(tmpDir, "opencode")
	if err := os.Symlink(profileDir, opencodeLink); err != nil {
		return fmt.Errorf("create ephemeral symlink: %w", err)
	}

	child := exec.Command(childArgs[0], childArgs[1:]...) //nolint:gosec
	child.Env = append(os.Environ(), "XDG_CONFIG_HOME="+tmpDir)
	child.Stdin = os.Stdin
	child.Stdout = cmd.OutOrStdout()
	child.Stderr = cmd.ErrOrStderr()

	if err := child.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Propagate the child's exit code without printing a redundant error.
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}
