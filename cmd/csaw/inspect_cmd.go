package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/inspect"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

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
