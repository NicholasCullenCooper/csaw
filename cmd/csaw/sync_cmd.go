package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
					printStaleAfterPull()
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
			printStaleAfterPull()
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

// printStaleAfterPull tells the user when a pull left mounted copies behind
// their sources, so the next step is obvious.
func printStaleAfterPull() {
	projectRoot, err := runtime.FindRepoRoot(".")
	if err != nil {
		return
	}
	state, err := workspace.ReadMountState(projectRoot)
	if err != nil || len(state.Entries) == 0 {
		return
	}
	stale := 0
	for _, status := range drift.InspectMountState(projectRoot, state, linkmode.Detect()) {
		if status.Issue == drift.IssueStale || status.Issue == drift.IssueStaleAndModified {
			stale++
		}
	}
	if stale > 0 {
		output.Infof("%d mounted copies are behind their sources; run csaw update to refresh them", stale)
	}
}
