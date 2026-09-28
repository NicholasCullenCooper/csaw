package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/drift"
	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/tui"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

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

			if len(state.Entries) > 0 {
				stale, modified, other := 0, 0, 0
				for _, status := range drift.InspectMountState(projectRoot, state, linkmode.Detect()) {
					switch {
					case status.Healthy:
					case status.LocallyModified():
						modified++
					case status.NeedsUpdate():
						stale++
					default:
						other++
					}
				}
				if stale+modified+other > 0 {
					var parts []string
					if stale > 0 {
						parts = append(parts, fmt.Sprintf("%d stale (csaw update)", stale))
					}
					if modified > 0 {
						parts = append(parts, fmt.Sprintf("%d edited locally (csaw check)", modified))
					}
					if other > 0 {
						parts = append(parts, fmt.Sprintf("%d unhealthy (csaw check)", other))
					}
					output.Label("drift:", output.Warn(strings.Join(parts, " · ")))
				}
			}

			// Surface uncommitted edits in source repos so users see the
			// edit-while-mounted state without having to run `pull` and
			// hit DirtySourceError.
			type dirtyReport struct {
				name  string
				files []sources.DirtyFile
			}
			var dirty []dirtyReport
			ctx := context.Background()
			for _, name := range names {
				files, err := manager.DirtyFiles(ctx, name)
				if err != nil {
					continue
				}
				if len(files) > 0 {
					dirty = append(dirty, dirtyReport{name: name, files: files})
				}
			}

			if len(dirty) > 0 {
				fmt.Println()
				output.Header("uncommitted edits in source checkouts")
				fmt.Println()
				for _, d := range dirty {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %s %s\n",
						output.Faint("•"),
						d.name,
						output.Faint(fmt.Sprintf("(%d %s)", len(d.files), pluralize("file", len(d.files)))),
					)
					// Show up to 5 file paths to give a sense of what changed
					// without flooding the terminal.
					const maxShown = 5
					for i, f := range d.files {
						if i >= maxShown {
							fmt.Fprintf(cmd.OutOrStdout(), "      %s\n",
								output.Faint(fmt.Sprintf("... +%d more", len(d.files)-maxShown)))
							break
						}
						fmt.Fprintf(cmd.OutOrStdout(), "      %s %s\n", output.Faint(f.Status), f.Path)
					}
				}
				fmt.Println()
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", tui.HintLine(
					"Next:",
					"csaw push <source> -m \"...\"  to share, or  csaw fork <path> --into personal  to keep private",
				))
			}

			return nil
		},
	}
}
