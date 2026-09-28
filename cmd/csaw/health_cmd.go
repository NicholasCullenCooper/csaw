package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/drift"
	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

func newCheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check mounted files for drift, local edits, and missing targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}

			projectRoot, err := targetProjectRoot()
			if err != nil {
				return err
			}

			state, err := workspace.ReadMountState(projectRoot)
			if err != nil {
				return err
			}

			var statuses []drift.Status
			if len(state.Entries) > 0 {
				statuses = drift.InspectMountState(projectRoot, state, linkmode.Detect())
			} else {
				links, err := workspace.FindMountedLinks(projectRoot, paths.Root)
				if err != nil {
					return err
				}
				statuses = drift.InspectLinks(links)
			}
			if len(statuses) == 0 {
				output.Muted("no mounted csaw files found")
				return nil
			}

			healthy := 0
			unhealthy := 0
			for _, status := range statuses {
				if status.Healthy {
					healthy++
					continue
				}
				unhealthy++
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %s %s\n",
					output.SymbolWarn,
					status.RelativePath,
					output.Warn(status.Issue),
				)
			}

			if unhealthy > 0 {
				fmt.Fprintln(cmd.OutOrStdout())
				output.Warnf("%d unhealthy, %d healthy", unhealthy, healthy)
				printDriftHints(cmd, statuses)
				return fmt.Errorf("%d mounted file(s) need attention", unhealthy)
			}

			output.Successf("%d files healthy", healthy)

			return nil
		},
	}
}

func newUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Repair or refresh mounted state",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := runtime.FindRepoRoot(".")
			if err != nil {
				return err
			}

			result, statuses, err := mount.Repair(projectRoot)
			if err != nil {
				return err
			}
			if result.Linked > 0 {
				if manager, err := newSourcesManager(); err == nil {
					if paths, err := runtime.ResolvePaths(); err == nil {
						if state, err := workspace.ReadMountState(projectRoot); err == nil {
							writeLock(manager, paths, projectRoot, state)
						}
					}
				}
			}

			unresolved := 0
			for _, status := range statuses {
				if !status.Healthy {
					unresolved++
				}
			}

			if result.Linked == 0 && unresolved == 0 {
				output.Infof("all mounted files are already up to date")
				return nil
			}

			if result.Linked > 0 {
				output.Successf("refreshed %d mounted file(s)", result.Linked)
			}
			if unresolved > 0 {
				output.Warnf("%d file(s) left untouched", unresolved)
				for _, status := range statuses {
					if !status.Healthy {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s %s %s\n", output.SymbolWarn, status.RelativePath, output.Warn(status.Issue))
					}
				}
				printDriftHints(cmd, statuses)
			}
			return nil
		},
	}
}

func newDiffCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <path>",
		Short: "Show the diff between a mounted file and its source target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			if _, err := os.Lstat(target); err != nil {
				return err
			}

			// Try resolving via symlink first
			resolvedTarget, err := os.Readlink(target)
			if err != nil {
				// Not a symlink — look up the source from mount state (hardlink case)
				projectRoot, prErr := runtime.FindRepoRoot(filepath.Dir(target))
				if prErr != nil {
					return fmt.Errorf("%s is not a mounted file", target)
				}
				state, stErr := workspace.ReadMountState(projectRoot)
				if stErr != nil {
					return fmt.Errorf("%s is not a mounted file", target)
				}
				absTarget, _ := filepath.Abs(target)
				found := false
				for _, entry := range state.Entries {
					entryPath := filepath.Join(projectRoot, filepath.FromSlash(entry.RelativePath))
					if runtime.PathsEqual(entryPath, absTarget) {
						resolvedTarget = entry.SourcePath
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("%s is not a mounted file", target)
				}
			} else if !filepath.IsAbs(resolvedTarget) {
				resolvedTarget = filepath.Join(filepath.Dir(target), resolvedTarget)
			}

			diffCmd := exec.Command("git", "diff", "--no-index", "--", target, resolvedTarget)
			diffCmd.Stdout = cmd.OutOrStdout()
			diffCmd.Stderr = cmd.ErrOrStderr()
			if err := diffCmd.Run(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
					return nil
				}
				return err
			}

			return nil
		},
	}
}

// printDriftHints explains what to do about locally modified and stale
// files: csaw never overwrites an edit, so the user has to choose.
func printDriftHints(cmd *cobra.Command, statuses []drift.Status) {
	modified := 0
	stale := 0
	for _, status := range statuses {
		if status.LocallyModified() {
			modified++
		} else if status.NeedsUpdate() {
			stale++
		}
	}
	if modified == 0 && stale == 0 {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout())
	if stale > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tui.HintLine("Refresh:", "csaw update  (rewrites missing and stale copies from their sources)"))
	}
	if modified > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tui.HintLine("Edited files:", "csaw diff <path> to review, csaw fork <source/path> to keep the edit, or csaw unmount <path> to drop it (edits are kept under .csaw-stash/modified/)"))
	}
}
