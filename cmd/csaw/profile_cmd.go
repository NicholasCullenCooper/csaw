package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/profiles"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
)

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
