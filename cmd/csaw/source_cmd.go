package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/profiles"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
)

func newSourceCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "source",
		Short: "Manage configured csaw sources",
	}

	addCmd := &cobra.Command{
		Use:   "add <name> <url-or-path>",
		Short: "Register a source in ~/.csaw/config.yml",
		Long: `Register a source. Accepts:
  - A local path (./team-config, ~/configs/team)
  - A full git URL (https://github.com/co/team-source.git, git@github.com:co/team.git)
  - Host shorthand: gh:org/repo, gl:org/repo, bb:org/repo (with optional #ref)

Examples:
  csaw source add team ./team-config
  csaw source add team https://github.com/co/team-source.git
  csaw source add team gh:co/team-source
  csaw source add team gh:co/team-source#main
  csaw source add team gh:co/team-source#v1.2.0`,
		Args: cobra.ExactArgs(2),
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
