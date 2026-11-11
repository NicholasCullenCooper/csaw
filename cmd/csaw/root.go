package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/git"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
)

var version = "dev"

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "csaw",
		Short:         "Mount AI workspace configuration into a project.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newConfigCommand())
	cmd.AddCommand(newSourceCommand())
	cmd.AddCommand(newProfileCommand())
	cmd.AddCommand(newUseCommand())
	cmd.AddCommand(newMountCommand())
	cmd.AddCommand(newUnmountCommand())
	cmd.AddCommand(newInspectCommand())
	cmd.AddCommand(newAuditCommand())
	cmd.AddCommand(newCheckCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newDiffCommand())
	cmd.AddCommand(newPullCommand())
	cmd.AddCommand(newPushCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newPinCommand())
	cmd.AddCommand(newUnpinCommand())
	cmd.AddCommand(newForkCommand())
	cmd.AddCommand(newPromoteCommand())
	cmd.AddCommand(newShowCommand())
	cmd.AddCommand(newHideCommand())
	cmd.AddCommand(newMCPCommand())
	cmd.AddCommand(newVendorCommand())
	cmd.AddCommand(newExportCommand())

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the current version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version)
		},
	}
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

func toolDisplayName(key string) string {
	return mount.ToolDisplayName(key)
}

func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func newSourcesManager() (sources.Manager, error) {
	paths, err := runtime.ResolvePaths()
	if err != nil {
		return sources.Manager{}, err
	}

	return sources.Manager{
		Paths: paths,
		Git:   git.ExecGit{},
	}, nil
}
