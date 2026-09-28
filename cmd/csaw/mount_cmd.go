package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

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
	modeFlag            string
	frozen              bool
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
	cmd.Flags().StringSliceVar(&options.toolsFlag, "tools", nil, "target tools (claude, codex)")
	cmd.Flags().StringVar(&options.modeFlag, "mode", "", "mount mode: copy (default) or symlink; overrides config mount_mode")
	cmd.Flags().BoolVar(&options.frozen, "frozen", false, "refuse to mount anything that differs from .csaw/lock.yml")
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

	configuredTools := options.toolsFlag
	if len(configuredTools) == 0 {
		cfg, _ := manager.Load()
		configuredTools = cfg.Tools
	}

	if len(configuredTools) == 0 && isInteractive() {
		detected := mount.ResolveTools(projectRoot, nil)
		hasRealTools := false
		for _, d := range detected {
			if d.Name != mount.StandardFallback.Name {
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

	tools := mount.ResolveTools(projectRoot, configuredTools)
	if !options.restore {
		entries = mount.ExpandToolTargets(entries, tools)
		for _, entry := range entries {
			for _, note := range entry.Notes {
				output.Warnf("%s", note)
			}
		}
	}

	mode, err := resolveMountMode(manager, options.modeFlag)
	if err != nil {
		return err
	}

	if options.frozen {
		if err := checkFrozen(manager, paths, projectRoot, entries, mode); err != nil {
			return err
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

	result, err := mount.ApplyWithOptions(projectRoot, paths, entries, promptConflictResolver{
		cmd:      cmd,
		forceAll: options.forceAll,
		skipAll:  options.skipConflicts,
	}, mount.ApplyOptions{Mode: mode})
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
	writeLock(manager, paths, projectRoot, displayState)

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
	if mode == mount.ModeSymlink {
		stats += " · symlink mode"
	}
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

func mountedToolDirCount(files []string) int {
	known := map[string]bool{}
	for _, dir := range mount.KnownTopLevelDirs() {
		known[dir] = true
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

// resolveMountMode picks the mount mode from the --mode flag, then the
// configured mount_mode, then the default (copy).
func resolveMountMode(manager sources.Manager, flag string) (mount.MountMode, error) {
	if flag != "" {
		return mount.ParseMountMode(flag)
	}
	cfg, err := manager.Load()
	if err != nil {
		return "", err
	}
	return mount.ParseMountMode(cfg.MountMode)
}
