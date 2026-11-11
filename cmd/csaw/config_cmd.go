package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/output"
)

func newConfigCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "config",
		Short: "View and set csaw configuration",
	}

	validKeys := []string{"tools", "default_fork_target", "mount_mode", "scanner"}

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
			case "mount_mode":
				mode, err := mount.ParseMountMode(value)
				if err != nil {
					return err
				}
				cfg.MountMode = string(mode)
			case "scanner":
				cfg.Scanner = strings.TrimSpace(value)
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
			case "mount_mode":
				mode, _ := mount.ParseMountMode(cfg.MountMode)
				fmt.Fprintln(cmd.OutOrStdout(), string(mode))
			case "scanner":
				if cfg.Scanner == "" {
					output.Muted("not set")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), cfg.Scanner)
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
			mode, _ := mount.ParseMountMode(cfg.MountMode)
			output.Label("mount mode:", string(mode))
			if cfg.Scanner != "" {
				output.Label("scanner:", cfg.Scanner)
			} else {
				output.Label("scanner:", output.Faint("not set (csaw vendor sync runs built-in checks only)"))
			}
			output.Label("sources:", fmt.Sprintf("%d", len(cfg.Sources)))
			return nil
		},
	})

	return rootCmd
}
