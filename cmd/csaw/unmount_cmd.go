package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/inspect"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
)

func newUnmountCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unmount [patterns...]",
		Short: "Remove mounted files from the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := runtime.FindRepoRoot(".")
			if err != nil {
				return err
			}

			result, err := mount.Unmount(projectRoot, mount.Selection{IncludePatterns: append([]string(nil), args...)})
			if err != nil {
				return err
			}
			if result.Removed == 0 && result.Restored == 0 {
				output.Infof("no mounted files matched the requested selection")
				return nil
			}

			fmt.Printf("%s %s\n", output.SymbolOK, inspect.RenderUnmountResult(result.Removed, result.Restored))
			for _, kept := range result.KeptModified {
				output.Warnf("kept your edits at %s", kept)
			}
			fmt.Printf("\n  %s\n", tui.HintLine("Remount:", "csaw mount --restore"))
			return nil
		},
	}
}
