package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/csaw-ai/csaw/internal/drift"
	"github.com/csaw-ai/csaw/internal/git"
	"github.com/csaw-ai/csaw/internal/inspect"
	"github.com/csaw-ai/csaw/internal/mount"
	"github.com/csaw-ai/csaw/internal/output"
	"github.com/csaw-ai/csaw/internal/runtime"
	"github.com/csaw-ai/csaw/internal/sources"
	"github.com/csaw-ai/csaw/internal/workspace"
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
	cmd.AddCommand(newSourceCommand())
	cmd.AddCommand(newMountCommand())
	cmd.AddCommand(newUnmountCommand())
	cmd.AddCommand(newInspectCommand())
	cmd.AddCommand(newCheckCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newDiffCommand())
	cmd.AddCommand(newPullCommand())
	cmd.AddCommand(newPushCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newShowCommand())
	cmd.AddCommand(newHideCommand())

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

func newSourceCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "source",
		Short: "Manage configured csaw sources",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "add <name> <url-or-path>",
		Short: "Register a source in ~/.csaw/config.yml",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			source, err := sources.NewSource(args[0], args[1])
			if err != nil {
				return err
			}

			if err := manager.Add(source); err != nil {
				return err
			}

			output.Successf("registered source %q", source.Name)
			return nil
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a source from ~/.csaw/config.yml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			if err := manager.Remove(args[0]); err != nil {
				return err
			}

			output.Successf("removed source %q", args[0])
			return nil
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			cfg, err := manager.Load()
			if err != nil {
				return err
			}

			if len(cfg.Sources) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no sources configured")
				return nil
			}

			items := append([]sources.Source(nil), cfg.Sources...)
			sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
			for _, source := range items {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\t%s\t%s\n",
					source.Name,
					source.Kind,
					source.CheckoutPath(manager.Paths),
				)
			}

			return nil
		},
	})

	return rootCmd
}

func newMountCommand() *cobra.Command {
	var excludes []string
	var profile string
	var includeIgnored bool
	var forceAll bool
	var skipConflicts bool
	var restore bool

	cmd := &cobra.Command{
		Use:   "mount [patterns...]",
		Short: "Mount registry files into the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := runtime.FindRepoRoot(".")
			if err != nil {
				return err
			}

			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			selection := mount.Selection{
				IncludePatterns: append([]string(nil), args...),
				ExcludePatterns: append([]string(nil), excludes...),
				Profile:         profile,
				IncludeIgnored:  includeIgnored,
			}

			var entries []mount.SourceEntry
			if restore {
				entries, err = entriesFromRestoreState(paths, projectRoot)
				if err != nil {
					return err
				}
				if len(entries) == 0 {
					return errors.New("no previous mount state found to restore")
				}
			} else {
				entries, err = collectMountEntries(manager, paths, selection)
				if err != nil {
					return err
				}
				if len(entries) == 0 {
					output.Warnf("no registry files matched the requested mount selection")
					return nil
				}
			}

			// Expand skill entries into tool-specific directories
			toolDirs := mount.DetectToolDirs(projectRoot)
			entries = mount.ExpandToolTargets(entries, toolDirs)

			result, err := mount.Apply(projectRoot, paths, entries, promptConflictResolver{
				cmd:      cmd,
				forceAll: forceAll,
				skipAll:  skipConflicts,
			})
			if err != nil {
				return err
			}

			if result.Linked == 0 && result.AlreadyLinked > 0 {
				output.Infof("all requested files were already mounted")
				return nil
			}

			output.Successf(
				"mounted %d file(s), stashed %d, skipped %d, already linked %d",
				result.Linked,
				result.Stashed,
				result.Skipped,
				result.AlreadyLinked,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "named profile to use for mount selection")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "exclude matching file or glob")
	cmd.Flags().BoolVar(&includeIgnored, "include-ignored", false, "include files hidden by .csawignore")
	cmd.Flags().BoolVar(&forceAll, "force", false, "overwrite conflicts and stash originals")
	cmd.Flags().BoolVar(&skipConflicts, "skip-conflicts", false, "skip files that conflict with existing paths")
	cmd.Flags().BoolVar(&restore, "restore", false, "restore the previous mount selection")

	return cmd
}

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

			output.Successf("unmounted %d file(s), restored %d original(s)", result.Removed, result.Restored)
			return nil
		},
	}
}

func newInspectCommand() *cobra.Command {
	var sourceName string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect configured sources and mounted state",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			if sourceName != "" {
				source, err := findNamedSource(manager, sourceName)
				if err != nil {
					return err
				}

				details, err := inspect.RenderSourceDetails(source, paths)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), details)

				previewPath := filepath.Join(source.CheckoutPath(paths), "AGENTS.md")
				if _, err := os.Stat(previewPath); err == nil {
					rendered, err := inspect.RenderMarkdownPreview(previewPath)
					if err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), rendered)
				}

				return nil
			}

			projectRoot, err := targetProjectRoot()
			if err != nil {
				return err
			}

			summary, err := inspect.BuildSummary(context.Background(), projectRoot, paths, manager)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), inspect.RenderSummary(summary))
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceName, "source", "", "show details for a single configured source")

	return cmd
}

func newCheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check mounted links for missing targets",
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
				statuses = drift.InspectMountState(projectRoot, state)
			} else {
				links, err := workspace.FindMountedLinks(projectRoot, paths.Root)
				if err != nil {
					return err
				}
				statuses = drift.InspectLinks(links)
			}
			if len(statuses) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no mounted csaw links found")
				return nil
			}

			unhealthy := 0
			for _, status := range statuses {
				if status.Healthy {
					fmt.Fprintf(cmd.OutOrStdout(), "ok\t%s\t%s\n", status.RelativePath, status.ResolvedTarget)
					continue
				}

				unhealthy++
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", status.Issue, status.RelativePath, status.ResolvedTarget)
			}

			if unhealthy > 0 {
				return fmt.Errorf("%d mounted link(s) need attention", unhealthy)
			}

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

			unresolved := 0
			for _, status := range statuses {
				if !status.Healthy && (status.Issue == drift.IssueMissingSource || status.Issue == drift.IssueReplacedLink) {
					unresolved++
				}
			}

			if result.Linked == 0 && unresolved == 0 {
				output.Infof("all mounted links are already healthy")
				return nil
			}

			if result.Linked > 0 {
				output.Successf("repaired %d drifted link(s)", result.Linked)
			}
			if unresolved > 0 {
				output.Warnf("%d link(s) remain unresolved", unresolved)
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
			info, err := os.Lstat(target)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("%s is not a symlink", target)
			}

			linkTarget, err := os.Readlink(target)
			if err != nil {
				return err
			}

			resolvedTarget := linkTarget
			if !filepath.IsAbs(resolvedTarget) {
				resolvedTarget = filepath.Join(filepath.Dir(target), linkTarget)
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

func newPullCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pull [source]",
		Short: "Clone or update configured remote sources",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			if len(args) == 1 {
				return manager.Pull(context.Background(), args[0])
			}

			return manager.PullAll(context.Background())
		},
	}
}

func newPushCommand() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push changes in the personal registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			err = manager.PushPersonal(context.Background(), message)
			if errors.Is(err, sources.ErrNothingToPush) {
				output.Infof("nothing to push in personal registry")
				return nil
			}
			if err != nil {
				return err
			}

			output.Successf("pushed personal registry changes")
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message for the personal registry push")
	return cmd
}

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show configured sources and mounted workspace state",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			projectRoot, err := targetProjectRoot()
			if err != nil {
				return err
			}

			cfg, err := manager.Load()
			if err != nil {
				return err
			}

			state, err := workspace.ReadMountState(projectRoot)
			if err != nil {
				return err
			}

			var names []string
			for _, source := range cfg.Sources {
				names = append(names, source.Name)
			}
			sort.Strings(names)

			fmt.Fprintf(cmd.OutOrStdout(), "project:\t%s\n", projectRoot)
			fmt.Fprintf(cmd.OutOrStdout(), "csaw home:\t%s\n", paths.Root)
			fmt.Fprintf(cmd.OutOrStdout(), "sources:\t%d", len(cfg.Sources))
			if len(names) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\t%s", strings.Join(names, ", "))
			}
			fmt.Fprintln(cmd.OutOrStdout())

			manifest, err := workspace.FileStateStore{}.ReadManifest(projectRoot)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "mounted:\t%d\n", len(state.Entries))
			fmt.Fprintf(cmd.OutOrStdout(), "stashed:\t%d\n", len(manifest))
			for _, entry := range state.Entries {
				fmt.Fprintf(cmd.OutOrStdout(), "link:\t%s\t%s\n", entry.RelativePath, entry.SourceName)
			}

			return nil
		},
	}
}

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
