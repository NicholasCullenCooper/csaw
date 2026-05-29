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

	"github.com/NicholasCullenCooper/csaw/internal/audit"
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
	var preset string
	var listPresets bool

	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new csaw registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if listPresets {
				fmt.Fprintln(cmd.OutOrStdout(), "Available presets:")
				fmt.Fprintln(cmd.OutOrStdout())
				for _, p := range registry.ListPresets() {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n      %s\n\n", p.Name, p.Description)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Use: csaw init --preset <name>\n")
				return nil
			}

			if adopt && preset != "" {
				return fmt.Errorf("--adopt and --preset are mutually exclusive (adopt copies existing project files; preset writes curated starters)")
			}

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
				initResult, err = registry.Init(context.Background(), git.ExecGit{}, dir, name, preset)
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
	cmd.Flags().StringVar(&preset, "preset", "", "scaffold from a curated preset (run --list-presets to see options)")
	cmd.Flags().BoolVar(&listPresets, "list-presets", false, "list available presets and exit")
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
					fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n", tui.HintLine("Run:", "csaw use "+selected))
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
				meta := string(source.Kind)
				if source.Priority != 0 {
					meta += fmt.Sprintf(", priority %d", source.Priority)
				}
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"  %s %s %s %s\n",
					output.Accent(source.Name),
					output.Faint("("+meta+")"),
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

func newProfileCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "profile",
		Aliases: []string{"profiles"},
		Short:   "List and inspect csaw profiles",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileList(cmd)
		},
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileList(cmd)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "show <profile>",
		Short: "Show the resolved profile recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}
			manager, err := newSourcesManager()
			if err != nil {
				return err
			}
			resolver, err := profileResolver(manager, paths)
			if err != nil {
				return err
			}

			profile, err := resolver.Resolve(args[0])
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), renderProfileDetails(profile))
			return nil
		},
	})

	return rootCmd
}

func runProfileList(cmd *cobra.Command) error {
	paths, err := runtime.ResolvePaths()
	if err != nil {
		return err
	}
	manager, err := newSourcesManager()
	if err != nil {
		return err
	}
	resolver, err := profileResolver(manager, paths)
	if err != nil {
		return err
	}
	allProfiles, err := resolver.All()
	if err != nil {
		return err
	}

	if len(allProfiles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no profiles found")
		fmt.Fprintln(cmd.OutOrStdout(), "add a source: csaw source add <name> <url-or-path>")
		return nil
	}

	fmt.Fprint(cmd.OutOrStdout(), renderProfileList(allProfiles))
	return nil
}

func profileResolver(manager sources.Manager, paths runtime.Paths) (*profiles.CatalogResolver, error) {
	catalog, err := manager.ExistingCatalog()
	if err != nil {
		return nil, err
	}
	return profiles.NewCatalogResolver(paths, catalog)
}

func renderProfileList(allProfiles map[string]profiles.Profile) string {
	var b strings.Builder
	b.WriteString("profiles\n\n")
	for _, name := range profiles.SortedNames(allProfiles) {
		profile := allProfiles[name]
		fmt.Fprintf(&b, "  %s", profile.Name)
		if profile.Description != "" {
			fmt.Fprintf(&b, "  %s", profile.Description)
		}
		fmt.Fprintf(&b, "\n      %s\n", profileStats(profile))
		fmt.Fprintf(&b, "      use: csaw use %s\n", profile.Name)
	}
	return b.String()
}

func renderProfileDetails(profile profiles.Profile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "profile: %s\n", profile.Name)
	if profile.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", profile.Description)
	}
	fmt.Fprintf(&b, "use: csaw use %s\n", profile.Name)
	fmt.Fprintf(&b, "include_ignored: %t\n", profile.IncludeIgnored)
	fmt.Fprintf(&b, "include_experimental: %t\n", profile.IncludeExperimental)
	writeProfilePatterns(&b, "include", profile.Include)
	writeProfilePatterns(&b, "exclude", profile.Exclude)
	return b.String()
}

func writeProfilePatterns(b *strings.Builder, label string, patterns []string) {
	if len(patterns) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", label)
	for _, pattern := range patterns {
		fmt.Fprintf(b, "  - %s\n", pattern)
	}
}

func profileStats(profile profiles.Profile) string {
	parts := []string{countLabel(len(profile.Include), "include")}
	if len(profile.Exclude) > 0 {
		parts = append(parts, countLabel(len(profile.Exclude), "exclude"))
	}
	if profile.IncludeIgnored {
		parts = append(parts, "includes ignored files")
	}
	if profile.IncludeExperimental {
		parts = append(parts, "includes experimental")
	}
	return strings.Join(parts, " · ")
}

func countLabel(count int, singular string) string {
	label := singular
	if count != 1 {
		label += "s"
	}
	return fmt.Sprintf("%d %s", count, label)
}

type mountRunOptions struct {
	excludes            []string
	profile             string
	includeIgnored      bool
	includeExperimental bool
	forceAll            bool
	skipConflicts       bool
	restore             bool
	keep                bool
	toolsFlag           []string
	kindsFlag           []string
	allowPicker         bool
}

func newUseCommand() *cobra.Command {
	options := mountRunOptions{}

	cmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "Activate a named csaw profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.profile = args[0]
			return runMountCommand(cmd, nil, options)
		},
	}

	addMountFlags(cmd, &options, false, false)
	return cmd
}

func newMountCommand() *cobra.Command {
	options := mountRunOptions{allowPicker: true}

	cmd := &cobra.Command{
		Use:   "mount [--profile name | patterns...]",
		Short: "Mount registry files into the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMountCommand(cmd, args, options)
		},
	}

	addMountFlags(cmd, &options, true, true)
	cmd.AddCommand(newMountProfileCommand())
	cmd.AddCommand(newMountPathsCommand())

	return cmd
}

func newMountProfileCommand() *cobra.Command {
	options := mountRunOptions{}

	cmd := &cobra.Command{
		Use:   "profile <name>",
		Short: "Mount a named profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.profile = args[0]
			return runMountCommand(cmd, nil, options)
		},
	}

	addMountFlags(cmd, &options, false, false)
	return cmd
}

func newMountPathsCommand() *cobra.Command {
	options := mountRunOptions{}

	cmd := &cobra.Command{
		Use:     "paths <pattern...>",
		Aliases: []string{"path"},
		Short:   "Mount registry paths or glob patterns",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMountCommand(cmd, args, options)
		},
	}

	addMountFlags(cmd, &options, false, false)
	return cmd
}

func addMountFlags(cmd *cobra.Command, options *mountRunOptions, includeProfile bool, includeRestore bool) {
	if includeProfile {
		cmd.Flags().StringVar(&options.profile, "profile", "", "named profile to use for mount selection")
	}
	cmd.Flags().StringArrayVar(&options.excludes, "exclude", nil, "exclude matching file or glob")
	cmd.Flags().BoolVar(&options.includeIgnored, "include-ignored", false, "include files matched by .csawignore patterns")
	cmd.Flags().BoolVar(&options.includeExperimental, "include-experimental", false, "include files under any 'experimental/' path segment (built-in convention)")
	cmd.Flags().BoolVar(&options.forceAll, "force", false, "overwrite conflicts and stash originals")
	cmd.Flags().BoolVar(&options.skipConflicts, "skip-conflicts", false, "skip files that conflict with existing paths")
	if includeRestore {
		cmd.Flags().BoolVar(&options.restore, "restore", false, "restore the previous mount selection")
	}
	cmd.Flags().BoolVar(&options.keep, "keep", false, "keep existing mounts instead of replacing them")
	cmd.Flags().StringSliceVar(&options.toolsFlag, "tools", nil, "target tools (e.g., claude,cursor)")
	cmd.Flags().StringSliceVar(&options.kindsFlag, "kind", nil, "filter by kind: agents, skills, rules, mcp, instructions (repeatable)")
}

func runMountCommand(cmd *cobra.Command, args []string, options mountRunOptions) error {
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

	profile := options.profile
	if options.allowPicker && profile == "" && len(args) == 0 && !options.restore {
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
			return nil
		}
		profile = picked
	}

	var kinds []mount.Kind
	for _, raw := range options.kindsFlag {
		kind, err := mount.ParseKind(raw)
		if err != nil {
			return err
		}
		kinds = append(kinds, kind)
	}

	selection := mount.Selection{
		IncludePatterns:     append([]string(nil), args...),
		ExcludePatterns:     append([]string(nil), options.excludes...),
		Profile:             profile,
		IncludeIgnored:      options.includeIgnored,
		IncludeExperimental: options.includeExperimental,
		Kinds:               kinds,
	}

	var entries []mount.SourceEntry
	if options.restore {
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
			return errors.New("no registry files matched the requested mount selection; check the source, profile, path, or kind filter")
		}
	}

	// Auto-unmount previous mount unless --keep is set. Preflight first so
	// noninteractive conflicts cannot clear the active context before failing.
	if !options.keep {
		currentState, err := workspace.ReadMountState(projectRoot)
		if err != nil {
			return err
		}
		if err := preflightAutoUnmountConflicts(projectRoot, entries, currentState, options.forceAll, options.skipConflicts); err != nil {
			return err
		}
		if len(currentState.Entries) > 0 {
			if _, err := mount.Unmount(projectRoot, mount.Selection{}); err != nil {
				return err
			}
		}
	}

	configuredTools := options.toolsFlag
	if len(configuredTools) == 0 {
		cfg, _ := manager.Load()
		configuredTools = cfg.Tools
	}

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
				cfg, _ := manager.Load()
				cfg.Tools = configuredTools
				_ = manager.Save(cfg)
			}
		}
	}

	toolDirs := mount.ResolveToolDirs(projectRoot, configuredTools)
	if !options.restore {
		entries = mount.ExpandToolTargets(entries, toolDirs)
	}

	result, err := mount.Apply(projectRoot, paths, entries, promptConflictResolver{
		cmd:      cmd,
		forceAll: options.forceAll,
		skipAll:  options.skipConflicts,
	})
	if err != nil {
		return err
	}

	if result.Linked == 0 && result.AlreadyLinked > 0 {
		output.Infof("all requested files were already mounted")
		return nil
	}

	displayState, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return err
	}

	var mountedFiles []string
	var sourceNames []string
	seenSources := make(map[string]bool)
	for _, entry := range displayState.Entries {
		mountedFiles = append(mountedFiles, entry.RelativePath)
		if !seenSources[entry.SourceName] {
			seenSources[entry.SourceName] = true
			sourceNames = append(sourceNames, entry.SourceName)
		}
	}

	displayFiles := mountedFiles
	if len(displayFiles) > 10 {
		displayFiles = append(displayFiles[:9], fmt.Sprintf("... and %d more", len(mountedFiles)-9))
	}

	stats := fmt.Sprintf("%d file(s) mounted", result.Linked)
	if result.Stashed > 0 {
		stats += fmt.Sprintf(" · %d stashed", result.Stashed)
	}
	if toolDirCount := mountedToolDirCount(mountedFiles); toolDirCount > 0 {
		stats += fmt.Sprintf(" · %d tool dirs", toolDirCount)
	}

	hints := []string{
		tui.HintLine("Inspect:", "csaw inspect"),
		tui.HintLine("Unmount:", "csaw unmount"),
	}

	fmt.Println(tui.MountPanel(displayFiles, sourceNames, stats, hints))
	return nil
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

func newAuditCommand() *cobra.Command {
	var strict bool
	var jsonOut bool
	var initPolicy bool
	var forceInit bool

	cmd := &cobra.Command{
		Use:   "audit [path]",
		Short: "Audit active AI workspace context against project policy",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}

			projectRoot, err := runtime.FindRepoRoot(target)
			if err != nil {
				return err
			}

			if initPolicy {
				if jsonOut || strict {
					return errors.New("--init cannot be combined with --json or --strict")
				}
				policyPath, created, err := audit.InitPolicy(projectRoot, audit.InitOptions{Force: forceInit})
				if err != nil {
					return err
				}
				action := "created"
				if !created {
					action = "updated"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", action, policyPath)
				return nil
			}

			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}

			report, err := audit.Run(projectRoot, paths)
			if err != nil {
				return err
			}

			if jsonOut {
				content, err := audit.RenderJSON(report)
				if err != nil {
					return err
				}
				if _, err := cmd.OutOrStdout().Write(content); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), audit.RenderText(report))
			}

			if report.Failed(strict) {
				return errors.New(report.FailureSummary(strict))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "fail on warnings as well as errors")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a JSON audit report")
	cmd.Flags().BoolVar(&initPolicy, "init", false, "write a starter .csaw/policy.yml")
	cmd.Flags().BoolVar(&forceInit, "force", false, "overwrite an existing audit policy when used with --init")
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
				if !status.Healthy {
					unresolved++
				}
			}

			if result.Linked == 0 && unresolved == 0 {
				output.Infof("all mounted links are already healthy")
				return nil
			}

			if result.Linked > 0 {
				output.Successf("repaired %d mounted link(s)", result.Linked)
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

			remapped, err := remapPinnedMountsToDefault(projectRoot, manager.Paths, source, manager.WorktreePath(source, projectRoot))
			if err != nil {
				return err
			}

			if err := manager.WorktreeRemove(context.Background(), source, projectRoot); err != nil {
				output.Warnf("could not remove worktree: %v", err)
			}

			state = pinning.Remove(state, sourceName)
			if err := pinning.Write(projectRoot, state); err != nil {
				return err
			}

			output.Successf("unpinned %s", sourceName)
			if remapped > 0 {
				output.Infof("updated %d mounted file(s) to the default checkout", remapped)
			}
			return nil
		},
	}
}

func remapPinnedMountsToDefault(projectRoot string, paths runtime.Paths, source sources.Source, worktreePath string) (int, error) {
	state, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return 0, err
	}
	if len(state.Entries) == 0 {
		return 0, nil
	}

	lm := linkmode.Detect()
	defaultRoot := source.CheckoutPath(paths)
	updated := 0

	for index := range state.Entries {
		entry := &state.Entries[index]
		if entry.SourceName != source.Name || !runtime.PathStartsWith(entry.SourcePath, worktreePath) {
			continue
		}

		sourceRel, err := filepath.Rel(worktreePath, entry.SourcePath)
		if err != nil {
			return updated, err
		}
		newSourcePath := filepath.Join(defaultRoot, sourceRel)
		if _, err := os.Stat(newSourcePath); err != nil {
			return updated, fmt.Errorf("cannot update %s after unpin: %w", entry.RelativePath, err)
		}

		targetPath := filepath.Join(projectRoot, filepath.FromSlash(entry.RelativePath))
		if _, err := os.Lstat(targetPath); err == nil {
			if !linkmode.IsLink(lm, targetPath, entry.SourcePath) {
				return updated, fmt.Errorf("cannot update %s after unpin: target is no longer the expected csaw link", entry.RelativePath)
			}
			if err := os.Remove(targetPath); err != nil {
				return updated, err
			}
		} else if !os.IsNotExist(err) {
			return updated, err
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return updated, err
		}
		if err := linkmode.Create(lm, newSourcePath, targetPath); err != nil {
			return updated, err
		}

		entry.SourcePath = newSourcePath
		if entry.Protected {
			hash, err := workspace.FileSHA256(newSourcePath)
			if err != nil {
				return updated, err
			}
			entry.SourceSHA256 = hash
		}
		updated++
	}

	if updated == 0 {
		return 0, nil
	}

	sort.Slice(state.Entries, func(i, j int) bool {
		return state.Entries[i].RelativePath < state.Entries[j].RelativePath
	})
	if err := workspace.WriteMountState(projectRoot, state); err != nil {
		return updated, err
	}
	if err := workspace.WriteRestoreState(paths, projectRoot, state); err != nil {
		return updated, err
	}
	return updated, nil
}

func mountedToolDirCount(files []string) int {
	known := map[string]bool{
		mount.StandardFallback.Dir: true,
	}
	for _, tool := range mount.ToolRegistry {
		known[tool.Dir] = true
	}

	seen := map[string]bool{}
	for _, file := range files {
		dir, _, ok := strings.Cut(filepath.ToSlash(file), "/")
		if !ok || !known[dir] {
			continue
		}
		seen[dir] = true
	}
	return len(seen)
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

			paths, err := runtime.ResolvePaths()
			if err != nil {
				return err
			}
			resolver, err := profiles.NewCatalogResolver(paths, catalog)
			if err != nil {
				return err
			}

			result, err := fork.Fork(args[0], into, catalog, resolver.ProtectedPaths())
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
	"claude":      "Claude Code",
	"cursor":      "Cursor",
	"opencode":    "OpenCode",
	"codex":       "Codex CLI",
	"antigravity": "Antigravity (Google)",
	"goose":       "Goose (AAIF / Linux Foundation)",
	"copilot":     "GitHub Copilot (VS Code + CLI)",
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
