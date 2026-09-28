package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/fork"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/profiles"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
)

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
