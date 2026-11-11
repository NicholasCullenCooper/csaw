package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/export"
	"github.com/NicholasCullenCooper/csaw/internal/lock"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/profiles"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
)

func newExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a source or profile as a plugin for Claude Code or Codex/ChatGPT",
		Long: `Export turns a csaw source (or one of its profiles) into a plugin the
supported tools' own channels install:

  claude-code    a Claude Code plugin (.claude-plugin/plugin.json, skills/,
                 agents/, hooks/hooks.json + scripts/, .mcp.json), plus a
                 marketplace to list several
  agent-plugins  an Agent Plugins 1.0 directory (plugin.json, skills/, mcp.json)
                 for Codex and ChatGPT

Rules and AGENTS.md are not plugin components in either format; keep mounting
them with csaw. Every omission is printed.`,
	}
	cmd.AddCommand(newExportPluginCommand())
	cmd.AddCommand(newExportMarketplaceCommand())
	return cmd
}

type exportOptions struct {
	format      string
	out         string
	name        string
	version     string
	description string
	owner       string
	force       bool
}

func newExportPluginCommand() *cobra.Command {
	var options exportOptions
	cmd := &cobra.Command{
		Use:   "plugin <source>[/<profile>]",
		Short: "Write one plugin directory from a source or profile",
		Example: `  csaw export plugin team --format claude-code --out ./dist/team-plugin
  csaw export plugin team/backend --format agent-plugins --out ./dist/team-backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := export.ParseFormat(options.format)
			if err != nil {
				return err
			}
			if options.out == "" {
				return fmt.Errorf("--out <dir> is required")
			}
			ctx, err := newExportContext()
			if err != nil {
				return err
			}
			sourceName, profileName, _ := strings.Cut(args[0], "/")
			input, err := ctx.input(sourceName, profileName, options)
			if err != nil {
				return err
			}
			result, err := export.Plugin(format, input, options.out, options.force)
			if err != nil {
				return err
			}
			output.Successf("wrote %s plugin %q to %s (%d files)", format, input.Name, options.out, len(result.Written))
			for _, note := range result.Omitted {
				output.Warnf("%s", note)
			}
			printExportHint(cmd, format, options.out)
			return nil
		},
	}
	cmd.Flags().StringVar(&options.format, "format", string(export.FormatClaudeCode), "claude-code or agent-plugins")
	cmd.Flags().StringVar(&options.out, "out", "", "directory to write the plugin into (required)")
	cmd.Flags().StringVar(&options.name, "name", "", "plugin name (default: <source> or <source>-<profile>)")
	cmd.Flags().StringVar(&options.version, "version", "", "plugin version (default: the source's checked-out commit)")
	cmd.Flags().StringVar(&options.description, "description", "", "plugin description (default: the profile description)")
	cmd.Flags().BoolVar(&options.force, "force", false, "replace the contents of --out if it is not empty")
	return cmd
}

func newExportMarketplaceCommand() *cobra.Command {
	var options exportOptions
	cmd := &cobra.Command{
		Use:   "marketplace <source>",
		Short: "Write a Claude Code marketplace with one plugin per profile of a source",
		Example: `  csaw export marketplace team --out ./dist/team-marketplace
  claude plugin marketplace add ./dist/team-marketplace`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.out == "" {
				return fmt.Errorf("--out <dir> is required")
			}
			ctx, err := newExportContext()
			if err != nil {
				return err
			}
			sourceName := args[0]
			profileNames, err := ctx.profilesOf(sourceName)
			if err != nil {
				return err
			}
			if len(profileNames) == 0 {
				return fmt.Errorf("source %q has no profiles to export", sourceName)
			}
			marketplaceName := options.name
			if marketplaceName == "" {
				marketplaceName = sourceName
			}
			var entries []export.MarketplaceEntry
			for _, profileName := range profileNames {
				input, err := ctx.input(sourceName, profileName, exportOptions{version: options.version})
				if err != nil {
					return err
				}
				pluginDir := filepath.Join(options.out, "plugins", input.Name)
				result, err := export.Plugin(export.FormatClaudeCode, input, pluginDir, options.force)
				if err != nil {
					return fmt.Errorf("profile %s: %w", profileName, err)
				}
				output.Successf("wrote plugin %q (%d files)", input.Name, len(result.Written))
				for _, note := range result.Omitted {
					output.Warnf("%s: %s", input.Name, note)
				}
				entries = append(entries, export.MarketplaceEntry{
					Name:        input.Name,
					Source:      "./plugins/" + input.Name,
					Description: input.Description,
				})
			}
			if err := export.Marketplace(marketplaceName, options.owner, entries, options.out); err != nil {
				return err
			}
			output.Successf("wrote marketplace %q with %d plugin(s) to %s", marketplaceName, len(entries), options.out)
			fmt.Fprintf(cmd.OutOrStdout(), "\n  %s\n", tui.HintLine("Register:", "claude plugin marketplace add "+options.out))
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tui.HintLine("Install:", "claude plugin install "+entries[0].Name+"@"+marketplaceName))
			return nil
		},
	}
	cmd.Flags().StringVar(&options.out, "out", "", "directory to write the marketplace into (required)")
	cmd.Flags().StringVar(&options.name, "name", "", "marketplace name (default: the source name)")
	cmd.Flags().StringVar(&options.owner, "owner", "", "owner name recorded in marketplace.json (default: csaw)")
	cmd.Flags().StringVar(&options.version, "version", "", "version for every exported plugin (default: the source's checked-out commit)")
	cmd.Flags().BoolVar(&options.force, "force", false, "replace existing plugin directories under --out")
	return cmd
}

type exportContext struct {
	manager  sources.Manager
	paths    runtime.Paths
	resolver *profiles.CatalogResolver
	roots    map[string]lock.SourceInfo
}

func newExportContext() (*exportContext, error) {
	paths, err := runtime.ResolvePaths()
	if err != nil {
		return nil, err
	}
	manager, err := newSourcesManager()
	if err != nil {
		return nil, err
	}
	resolver, err := profileResolver(manager, paths)
	if err != nil {
		return nil, err
	}
	projectRoot, _ := runtime.FindRepoRoot(".")
	roots, err := lock.SourceInfos(context.Background(), manager.Git, paths, projectRoot)
	if err != nil {
		return nil, err
	}
	return &exportContext{manager: manager, paths: paths, resolver: resolver, roots: roots}, nil
}

// profilesOf lists the profile names defined by one source, sorted.
func (c *exportContext) profilesOf(sourceName string) ([]string, error) {
	all, err := c.resolver.All()
	if err != nil {
		return nil, err
	}
	var names []string
	for qualified := range all {
		if source, profile, ok := strings.Cut(qualified, "/"); ok && source == sourceName {
			names = append(names, profile)
		}
	}
	sort.Strings(names)
	return names, nil
}

// input gathers a source's (or profile's) entries and manifest metadata.
func (c *exportContext) input(sourceName string, profileName string, options exportOptions) (export.Input, error) {
	if _, err := findNamedSource(c.manager, sourceName); err != nil {
		return export.Input{}, err
	}
	selection := mount.Selection{IncludePatterns: []string{sourceName + "/**"}}
	description := options.description
	if profileName != "" {
		qualified := sourceName + "/" + profileName
		profile, err := c.resolver.Resolve(qualified)
		if err != nil {
			return export.Input{}, err
		}
		selection = mount.Selection{Profile: qualified}
		if description == "" {
			description = profile.Description
		}
	}
	entries, err := collectMountEntries(c.manager, c.paths, selection)
	if err != nil {
		return export.Input{}, err
	}
	var own []mount.SourceEntry
	for _, entry := range entries {
		if entry.SourceName == sourceName {
			own = append(own, entry)
		}
	}
	if len(own) == 0 {
		return export.Input{}, fmt.Errorf("nothing to export from %s", sourceName)
	}

	name := options.name
	if name == "" {
		name = sourceName
		if profileName != "" {
			name += "-" + profileName
		}
		name = strings.NewReplacer(" ", "-", "@", "-", ":", "-", "/", "-").Replace(name)
	}
	version := options.version
	if version == "" {
		if info, ok := c.roots[sourceName]; ok && len(info.RefResolved) >= 12 {
			version = "0.0.0+" + info.RefResolved[:12]
		}
	}
	return export.Input{Name: name, Version: version, Description: description, Entries: own}, nil
}

func printExportHint(cmd *cobra.Command, format export.Format, out string) {
	fmt.Fprintln(cmd.OutOrStdout())
	switch format {
	case export.FormatClaudeCode:
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tui.HintLine("Validate:", "claude plugin validate "+out))
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tui.HintLine("Try it:", "claude --plugin-dir "+out))
	case export.FormatAgentPlugins:
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tui.HintLine("Next:", "list "+out+" in a Codex marketplace (.agents/plugins/marketplace.json) or share the directory"))
	}
}
