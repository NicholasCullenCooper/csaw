package main

import (
	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

func newShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <path>...",
		Short: "Make mounted files visible to git (remove from git exclude)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := targetProjectRoot()
			if err != nil {
				return err
			}

			for _, path := range args {
				removed, err := workspace.RemoveExclusion(projectRoot, path)
				if err != nil {
					return err
				}
				if !removed {
					if workspace.IsGitIgnored(projectRoot, path) {
						file, pattern := workspace.GitIgnoreSource(projectRoot, path)
						output.Infof("%s is hidden by .gitignore (%s: %s), not by csaw", path, file, pattern)
					} else {
						output.Infof("%s was not in git exclude", path)
					}
				} else {
					// Check if still ignored by .gitignore
					file, pattern := workspace.GitIgnoreSource(projectRoot, path)
					if file != "" {
						output.Warnf("%s removed from git exclude, but still ignored by %s (pattern: %s)", path, file, pattern)
					} else {
						output.Successf("%s is now visible to git", path)
					}
				}
			}

			return nil
		},
	}
}

func newHideCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "hide <path>...",
		Short: "Hide mounted files from git (add to git exclude)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := targetProjectRoot()
			if err != nil {
				return err
			}

			for _, path := range args {
				if workspace.IsGitIgnored(projectRoot, path) {
					output.Infof("%s is already hidden by .gitignore", path)
					continue
				}

				added, err := workspace.AddExclusion(projectRoot, path)
				if err != nil {
					return err
				}
				if !added {
					output.Infof("%s was already in git exclude", path)
				} else {
					output.Successf("%s is now hidden from git", path)
				}
			}

			return nil
		},
	}
}
