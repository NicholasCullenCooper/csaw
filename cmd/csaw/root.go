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

	"github.com/NicholasCullenCooper/csaw/internal/drift"
	"github.com/NicholasCullenCooper/csaw/internal/fork"
	"github.com/NicholasCullenCooper/csaw/internal/git"
	"github.com/NicholasCullenCooper/csaw/internal/inspect"
	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/pinning"
	"github.com/NicholasCullenCooper/csaw/internal/profiles"
	"github.com/NicholasCullenCooper/csaw/internal/registry"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
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
	cmd.AddCommand(newMountCommand())
	cmd.AddCommand(newUnmountCommand())
	cmd.AddCommand(newInspectCommand())
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

func newInitCommand() *cobra.Command {
	var name string
	var adopt bool

	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new csaw registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			var initResult registry.InitResult
			var adoptedFiles []string

			if adopt {
				projectRoot, err := runtime.FindRepoRoot(".")
				if err != nil {
					return fmt.Errorf("--adopt requires being inside a git repository")
				}
				adoptResult, err := registry.InitWithAdopt(context.Background(), git.ExecGit{}, dir, name, projectRoot)
				if err != nil {
					return err
				}
				initResult = adoptResult.InitResult
				adoptedFiles = adoptResult.AdoptedFiles
			} else {
				var err error
				initResult, err = registry.Init(context.Background(), git.ExecGit{}, dir, name)
				if err != nil {
					return err
				}
			}

			output.Successf("initialized registry %q at %s", initResult.Name, initResult.Path)

			if len(adoptedFiles) > 0 {
				var lines []string
				for _, f := range adoptedFiles {
					lines = append(lines, fmt.Sprintf(" %s %s", output.SymbolOK, f))
				}
				fmt.Println(tui.ResultPanel(
					fmt.Sprintf("adopted %d files", len(adoptedFiles)),
					lines,
					nil,
				))
			}

			if !isInteractive() {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n", tui.HintLine("Next:", "csaw source add "+initResult.Name+" "+initResult.Path))
				return nil
			}

			// Offer to register as a source
			wizResult, err := tui.RunWizard([]tui.Step{
				{
					Kind:    tui.StepConfirm,
					Key:     "register",
					Title:   "Register as a source?",
					Default: "y",
				},
			})
			if err != nil || wizResult.Aborted || wizResult.Values["register"] != "y" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n", tui.HintLine("Later:", "csaw source add "+initResult.Name+" "+initResult.Path))
				return nil
			}

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			source := sources.Source{
				Name:     initResult.Name,
				Kind:     sources.KindLocal,
				Path:     initResult.Path,
				Priority: 10,
			}
			if err := manager.Add(source); err != nil {
				return err
			}

			output.Successf("registered source %q with priority 10", initResult.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "registry name (defaults to directory name)")
	cmd.Flags().BoolVar(&adopt, "adopt", false, "adopt existing AI config files from the current project")
	return cmd
}

func newConfigCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "config",
		Short: "View and set csaw configuration",
	}

	validKeys := []string{"tools", "default_fork_target"}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			cfg, err := manager.Load()
			if err != nil {
				return err
			}

			switch key {
			case "tools":
				tools := strings.Split(value, ",")
				for _, t := range tools {
					t = strings.TrimSpace(t)
					if _, ok := mount.ToolRegistry[t]; !ok {
						return fmt.Errorf("unknown tool %q; valid tools: %s", t, strings.Join(mount.AllToolNames(), ", "))
					}
				}
				cfg.Tools = tools
			case "default_fork_target":
				if _, err := manager.Get(value); err != nil {
					return fmt.Errorf("source %q not found; add it first with: csaw source add %s <url>", value, value)
				}
				cfg.DefaultForkTarget = value
			default:
				return fmt.Errorf("unknown config key %q; valid keys: %s", key, strings.Join(validKeys, ", "))
			}

			if err := manager.Save(cfg); err != nil {
				return err
			}
			output.Successf("set %s = %s", key, value)
			return nil
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			cfg, err := manager.Load()
			if err != nil {
				return err
			}

			switch key {
			case "tools":
				if len(cfg.Tools) == 0 {
					output.Muted("not set")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cfg.Tools, ","))
				}
			case "default_fork_target":
				if cfg.DefaultForkTarget == "" {
					output.Muted("not set")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), cfg.DefaultForkTarget)
				}
			default:
				return fmt.Errorf("unknown config key %q; valid keys: %s", key, strings.Join(validKeys, ", "))
			}
			return nil
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Show all configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			cfg, err := manager.Load()
			if err != nil {
				return err
			}

			output.Header("csaw config")
			fmt.Println()
			if len(cfg.Tools) > 0 {
				output.Label("tools:", strings.Join(cfg.Tools, ", "))
			} else {
				output.Label("tools:", output.Faint("not set"))
			}
			if cfg.DefaultForkTarget != "" {
				output.Label("fork target:", cfg.DefaultForkTarget)
			} else {
				output.Label("fork target:", output.Faint("not set"))
			}
			output.Label("sources:", fmt.Sprintf("%d", len(cfg.Sources)))
			return nil
		},
	})

	return rootCmd
}

func newSourceCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "source",
		Short: "Manage configured csaw sources",
	}

	addCmd := &cobra.Command{
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

			priority, _ := cmd.Flags().GetInt("priority")
			source.Priority = priority

			if err := manager.Add(source); err != nil {
				return err
			}

			output.Successf("registered source %q", source.Name)

			// Auto-pull remote sources
			if source.Kind == sources.KindRemote {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s cloning...\n", output.Faint("→"))
				if err := manager.Pull(context.Background(), source.Name, false); err != nil {
					return err
				}
				output.Successf("cloned %s", source.Name)
			}

			// Show available profiles and offer to mount
			if isInteractive() {
				paths, err := runtime.ResolvePaths()
				if err != nil {
					return nil
				}
				catalog, err := manager.ExistingCatalog()
				if err != nil {
					return nil
				}

				resolver, err := profiles.NewCatalogResolver(paths, catalog)
				if err != nil {
					return nil
				}
				allProfiles, err := resolver.All()
				if err != nil || len(allProfiles) == 0 {
					return nil
				}

				// Build picker items for profiles from this source
				items := []tui.PickerItem{{Name: "skip", Description: "I'll mount later"}}
				for _, name := range profiles.SortedNames(allProfiles) {
					items = append(items, tui.PickerItem{
						Name:        name,
						Description: allProfiles[name].Description,
					})
				}

				fmt.Println()
				wizResult, err := tui.RunWizard([]tui.Step{
					{
						Kind:    tui.StepSelect,
						Key:     "profile",
						Title:   "Mount a profile now?",
						Options: items,
					},
				})
				if err != nil || wizResult.Aborted {
					return nil
				}

				selected := wizResult.Values["profile"]
				if selected != "" && selected != "skip" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n", tui.HintLine("Run:", "csaw mount --profile "+selected))
				}
			}

			return nil
		},
	}
	addCmd.Flags().Int("priority", 0, "source priority (higher wins on conflict)")
	rootCmd.AddCommand(addCmd)

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
				output.Muted("no sources configured")
				return nil
			}

			items := append([]sources.Source(nil), cfg.Sources...)
			sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
			for _, source := range items {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"  %s %s %s %s\n",
					output.Accent(source.Name),
					output.Faint("("+string(source.Kind)+")"),
					output.Faint("→"),
					source.CheckoutPath(manager.Paths),
				)
			}

			return nil
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "clone <name> <dir>",
		Short: "Clone a remote source to a local directory for contributing",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, dir := args[0], args[1]

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			source, err := manager.Get(name)
			if err != nil {
				return err
			}

			if source.Kind != sources.KindRemote {
				return fmt.Errorf("source %q is already local at %s", name, source.Path)
			}

			absDir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}

			// Clone to the specified directory
			if _, err := manager.Git.Run(context.Background(), ".", "clone", source.URL, absDir); err != nil {
				return err
			}

			// Remove old managed checkout
			oldCheckout := source.CheckoutPath(manager.Paths)
			if _, err := os.Stat(oldCheckout); err == nil {
				os.RemoveAll(oldCheckout)
			}

			// Update source to point to local clone
			if err := manager.Remove(name); err != nil {
				return err
			}
			localSource := sources.Source{
				Name:     name,
				Kind:     sources.KindLocal,
				Path:     absDir,
				Priority: source.Priority,
			}
			if err := manager.Add(localSource); err != nil {
				return err
			}

			output.Successf("cloned %s to %s", name, absDir)
			output.Infof("source %q now points to local clone", name)
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
	var keep bool
	var toolsFlag []string

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

			// If no profile, no patterns, and not restoring — show interactive picker
			if profile == "" && len(args) == 0 && !restore {
				// Check if any sources are configured
				cfg, err := manager.Load()
				if err != nil {
					return err
				}
				if len(cfg.Sources) == 0 {
					if !isInteractive() {
						return errors.New("no sources configured; run: csaw source add <name> <url>")
					}
					fmt.Println(tui.ResultPanel("welcome to csaw", []string{
						output.Faint("No sources configured yet. Get started:"),
					}, []string{
						tui.HintLine("Create a registry:", "csaw init ~/my-ai-config"),
						tui.HintLine("Add a team source:", "csaw source add team <git-url>"),
					}))
					return nil
				}

				picked, err := pickProfile(manager, paths)
				if err != nil {
					return err
				}
				if picked == "" {
					return nil // user cancelled
				}
				profile = picked
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

			// Auto-unmount previous mount unless --keep is set
			if !keep {
				currentState, err := workspace.ReadMountState(projectRoot)
				if err != nil {
					return err
				}
				if len(currentState.Entries) > 0 {
					if _, err := mount.Unmount(projectRoot, mount.Selection{}); err != nil {
						return err
					}
				}
			}

			// Resolve tool directories: CLI flag > config > auto-detect
			configuredTools := toolsFlag
			if len(configuredTools) == 0 {
				cfg, _ := manager.Load()
				configuredTools = cfg.Tools
			}

			// If no tools configured and interactive, ask
			if len(configuredTools) == 0 && isInteractive() {
				detected := mount.ResolveToolDirs(projectRoot, nil)
				hasRealTools := false
				for _, d := range detected {
					if d.Dir != ".agents" {
						hasRealTools = true
						break
					}
				}
				if !hasRealTools {
					items := make([]tui.MultiSelectItem, 0, len(mount.ToolRegistry))
					for _, name := range mount.AllToolNames() {
						items = append(items, tui.MultiSelectItem{
							Key:   name,
							Label: toolDisplayName(name),
						})
					}
					msResult, err := tui.RunMultiSelect("Which AI tools do you use?", items)
					if err == nil && !msResult.Aborted && len(msResult.Selected) > 0 {
						configuredTools = msResult.Selected
						// Save to config for future mounts
						cfg, _ := manager.Load()
						cfg.Tools = configuredTools
						_ = manager.Save(cfg)
					}
				}
			}

			toolDirs := mount.ResolveToolDirs(projectRoot, configuredTools)
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

			// Collect mounted file paths for the panel
			var mountedFiles []string
			var sourceNames []string
			seenSources := make(map[string]bool)
			for _, entry := range entries {
				mountedFiles = append(mountedFiles, entry.RelativePath)
				if !seenSources[entry.SourceName] {
					seenSources[entry.SourceName] = true
					sourceNames = append(sourceNames, entry.SourceName)
				}
			}

			// Limit displayed files
			displayFiles := mountedFiles
			if len(displayFiles) > 10 {
				displayFiles = append(displayFiles[:9], fmt.Sprintf("... and %d more", len(mountedFiles)-9))
			}

			stats := fmt.Sprintf("%d files mounted", result.Linked)
			if result.Stashed > 0 {
				stats += fmt.Sprintf(" · %d stashed", result.Stashed)
			}
			if len(toolDirs) > 0 {
				stats += fmt.Sprintf(" · %d tool dirs", len(toolDirs))
			}

			hints := []string{
				tui.HintLine("Inspect:", "csaw inspect"),
				tui.HintLine("Unmount:", "csaw unmount"),
			}

			fmt.Println(tui.MountPanel(displayFiles, sourceNames, stats, hints))
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "named profile to use for mount selection")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "exclude matching file or glob")
	cmd.Flags().BoolVar(&includeIgnored, "include-ignored", false, "include files hidden by .csawignore")
	cmd.Flags().BoolVar(&includeIgnored, "include-experimental", false, "include experimental skills (alias for --include-ignored)")
	cmd.Flags().BoolVar(&forceAll, "force", false, "overwrite conflicts and stash originals")
	cmd.Flags().BoolVar(&skipConflicts, "skip-conflicts", false, "skip files that conflict with existing paths")
	cmd.Flags().BoolVar(&restore, "restore", false, "restore the previous mount selection")
	cmd.Flags().BoolVar(&keep, "keep", false, "keep existing mounts instead of replacing them")
	cmd.Flags().StringSliceVar(&toolsFlag, "tools", nil, "target tools (e.g., claude,cursor)")

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

			fmt.Printf("%s %s\n", output.SymbolOK, inspect.RenderUnmountResult(result.Removed, result.Restored))
			fmt.Printf("\n  %s\n", tui.HintLine("Remount:", "csaw mount --restore"))
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
				statuses = drift.InspectMountState(projectRoot, state, linkmode.Detect())
			} else {
				links, err := workspace.FindMountedLinks(projectRoot, paths.Root)
				if err != nil {
					return err
				}
				statuses = drift.InspectLinks(links)
			}
			if len(statuses) == 0 {
				output.Muted("no mounted csaw links found")
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
				return fmt.Errorf("%d mounted link(s) need attention", unhealthy)
			}

			output.Successf("%d links healthy", healthy)

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

func newPullCommand() *cobra.Command {
	var stash bool

	cmd := &cobra.Command{
		Use:   "pull [source]",
		Short: "Clone or update configured remote sources",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			if len(args) == 1 {
				err := manager.Pull(context.Background(), args[0], stash)
				if err == nil {
					output.Successf("pulled %s", args[0])
					return nil
				}
				return handlePullError(cmd, err)
			}

			results, err := manager.PullAll(context.Background(), stash)
			if err != nil {
				return err
			}

			var hasErrors bool
			for _, r := range results {
				if r.Err == nil {
					output.Successf("pulled %s", r.Source)
					continue
				}
				hasErrors = true
				var dirtyErr *sources.DirtySourceError
				var divErr *sources.DivergedSourceError
				if errors.As(r.Err, &dirtyErr) {
					output.Warnf("%s has uncommitted changes (use --stash)", r.Source)
				} else if errors.As(r.Err, &divErr) {
					output.Warnf("%s has diverged (%d local, %d remote commits)", divErr.Source, divErr.Ahead, divErr.Behind)
				} else {
					output.Errorf("%s: %v", r.Source, r.Err)
				}
			}

			if hasErrors {
				return fmt.Errorf("some sources failed to pull")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&stash, "stash", false, "stash uncommitted changes before pulling")
	return cmd
}

func newPushCommand() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "push [source]",
		Short: "Commit and push changes in a source registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			var name string
			if len(args) == 1 {
				name = args[0]
			} else {
				cfg, err := manager.Load()
				if err != nil {
					return err
				}
				var dirty []string
				for _, source := range cfg.Sources {
					root := source.CheckoutPath(manager.Paths)
					if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
						continue
					}
					status, err := manager.Git.Run(context.Background(), root, "status", "--porcelain")
					if err != nil {
						continue
					}
					if strings.TrimSpace(status) != "" {
						dirty = append(dirty, source.Name)
					}
				}
				switch len(dirty) {
				case 0:
					output.Infof("nothing to push")
					return nil
				case 1:
					name = dirty[0]
				default:
					return fmt.Errorf("multiple sources have changes: %s\nSpecify one: csaw push <source>", strings.Join(dirty, ", "))
				}
			}

			err = manager.Push(context.Background(), name, message)
			if errors.Is(err, sources.ErrNothingToPush) {
				output.Infof("nothing to push in %s", name)
				return nil
			}
			if err != nil {
				return err
			}

			output.Successf("pushed %s", name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message")
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

			output.Header("csaw status")
			fmt.Println()
			output.Label("project:", projectRoot)
			output.Label("csaw home:", paths.Root)

			sourcesSummary := fmt.Sprintf("%d", len(cfg.Sources))
			if len(names) > 0 {
				sourcesSummary += " " + output.Faint("("+strings.Join(names, ", ")+")")
			}
			output.Label("sources:", sourcesSummary)

			manifest, err := workspace.FileStateStore{}.ReadManifest(projectRoot)
			if err != nil {
				return err
			}

			mountedSummary := fmt.Sprintf("%d", len(state.Entries))
			if len(manifest) > 0 {
				mountedSummary += output.Faint(fmt.Sprintf(" · %d stashed", len(manifest)))
			}
			output.Label("mounted:", mountedSummary)

			return nil
		},
	}
}

func newPinCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pin <source>@<ref>",
		Short: "Pin a source to a branch or tag for this project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.SplitN(args[0], "@", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("usage: csaw pin <source>@<ref>")
			}
			sourceName, ref := parts[0], parts[1]

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			source, err := manager.Get(sourceName)
			if err != nil {
				return err
			}

			if source.Kind != sources.KindRemote {
				return fmt.Errorf("pinning is only supported for remote sources (use git directly for local sources)")
			}

			projectRoot, err := runtime.FindRepoRoot(".")
			if err != nil {
				return err
			}

			if _, err := manager.WorktreeCheckout(context.Background(), source, ref, projectRoot); err != nil {
				return err
			}

			state, err := pinning.Read(projectRoot)
			if err != nil {
				return err
			}
			state = pinning.Set(state, sourceName, ref)
			if err := pinning.Write(projectRoot, state); err != nil {
				return err
			}

			output.Successf("pinned %s to %s", sourceName, ref)
			return nil
		},
	}
}

func newUnpinCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unpin <source>",
		Short: "Unpin a source, returning to the default branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceName := args[0]

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			source, err := manager.Get(sourceName)
			if err != nil {
				return err
			}

			projectRoot, err := runtime.FindRepoRoot(".")
			if err != nil {
				return err
			}

			state, err := pinning.Read(projectRoot)
			if err != nil {
				return err
			}

			if _, ok := pinning.Get(state, sourceName); !ok {
				output.Infof("%s is not pinned", sourceName)
				return nil
			}

			if err := manager.WorktreeRemove(context.Background(), source, projectRoot); err != nil {
				output.Warnf("could not remove worktree: %v", err)
			}

			state = pinning.Remove(state, sourceName)
			if err := pinning.Write(projectRoot, state); err != nil {
				return err
			}

			output.Successf("unpinned %s", sourceName)
			return nil
		},
	}
}

func newForkCommand() *cobra.Command {
	var into string

	cmd := &cobra.Command{
		Use:   "fork <source/path>",
		Short: "Copy a file from one source into another for personal editing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			if into == "" {
				cfg, err := manager.Load()
				if err != nil {
					return err
				}
				into = cfg.DefaultForkTarget
			}
			if into == "" {
				return fmt.Errorf("specify target with --into or set default_fork_target in config.yml")
			}

			catalog, err := manager.ExistingCatalog()
			if err != nil {
				return err
			}

			result, err := fork.Fork(args[0], into, catalog)
			if err != nil {
				return err
			}

			output.Successf("forked %s/%s into %s", result.FromSource, result.RelativePath, result.IntoSource)
			return nil
		},
	}

	cmd.Flags().StringVar(&into, "into", "", "target source to fork into")
	return cmd
}

func newPromoteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "promote <source/skills/experimental/name>",
		Short: "Promote an experimental skill to stable",
		Long: `Move a skill from skills/experimental/ to skills/ in a source registry.

Example:
  csaw promote personal/skills/experimental/debug-strategy

This moves skills/experimental/debug-strategy/ to skills/debug-strategy/
in the personal source.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.SplitN(args[0], "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("usage: csaw promote <source/skills/experimental/name>")
			}
			sourceName, relPath := parts[0], parts[1]

			// Validate it's an experimental skill path
			if !strings.HasPrefix(relPath, "skills/experimental/") {
				return fmt.Errorf("can only promote from skills/experimental/; got %q", relPath)
			}

			skillName := strings.TrimPrefix(relPath, "skills/experimental/")
			skillName = strings.TrimSuffix(skillName, "/")
			if skillName == "" {
				return fmt.Errorf("missing skill name")
			}

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			source, err := manager.Get(sourceName)
			if err != nil {
				return err
			}

			root := source.CheckoutPath(manager.Paths)
			srcDir := filepath.Join(root, "skills", "experimental", skillName)
			dstDir := filepath.Join(root, "skills", skillName)

			if _, err := os.Stat(srcDir); os.IsNotExist(err) {
				return fmt.Errorf("experimental skill not found: %s", srcDir)
			}
			if _, err := os.Stat(dstDir); err == nil {
				return fmt.Errorf("stable skill already exists: %s", dstDir)
			}

			if err := os.Rename(srcDir, dstDir); err != nil {
				return err
			}

			output.Successf("promoted %s from experimental to stable", skillName)
			fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n", tui.HintLine("Push:", "csaw push "+sourceName+" -m \"promote "+skillName+"\""))
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

var toolDisplayNames = map[string]string{
	"claude":   "Claude Code",
	"cursor":   "Cursor",
	"opencode": "OpenCode",
	"codex":    "Codex",
	"windsurf": "Windsurf",
}

func toolDisplayName(key string) string {
	if name, ok := toolDisplayNames[key]; ok {
		return name
	}
	return key
}

func handlePullError(cmd *cobra.Command, err error) error {
	var dirtyErr *sources.DirtySourceError
	var divErr *sources.DivergedSourceError

	switch {
	case errors.As(err, &dirtyErr):
		output.Warnf("%s has uncommitted changes", dirtyErr.Source)
		fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n", tui.HintLine("Commit:", "cd "+dirtyErr.Path+" && git add -A && git commit -m \"...\""))
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tui.HintLine("Or stash:", "csaw pull "+dirtyErr.Source+" --stash"))
		return fmt.Errorf("pull aborted for %s", dirtyErr.Source)

	case errors.As(err, &divErr):
		output.Warnf("%s has diverged (%d local, %d remote commits)", divErr.Source, divErr.Ahead, divErr.Behind)
		fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n", tui.HintLine("Resolve:", "cd "+divErr.Path+" && git pull --rebase"))
		return fmt.Errorf("pull aborted for %s", divErr.Source)

	default:
		return err
	}
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
