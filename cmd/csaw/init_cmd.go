package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/git"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/registry"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
)

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
