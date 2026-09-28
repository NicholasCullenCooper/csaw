package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/output"
	"github.com/NicholasCullenCooper/csaw/internal/pinning"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

func newPinCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pin <source>@<ref>",
		Short: "Pin a source to a branch or tag for this project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.SplitN(args[0], "@", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("usage: csaw pin <source>@<ref>")
			}
			sourceName, ref := parts[0], parts[1]

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			source, err := manager.Get(sourceName)
			if err != nil {
				return err
			}

			if source.Kind != sources.KindRemote {
				return fmt.Errorf("pinning is only supported for remote sources (use git directly for local sources)")
			}

			projectRoot, err := runtime.FindRepoRoot(".")
			if err != nil {
				return err
			}

			if _, err := manager.WorktreeCheckout(context.Background(), source, ref, projectRoot); err != nil {
				return err
			}

			state, err := pinning.Read(projectRoot)
			if err != nil {
				return err
			}
			state = pinning.Set(state, sourceName, ref)
			if err := pinning.Write(projectRoot, state); err != nil {
				return err
			}

			output.Successf("pinned %s to %s", sourceName, ref)
			return nil
		},
	}
}

func newUnpinCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unpin <source>",
		Short: "Unpin a source, returning to the default branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceName := args[0]

			manager, err := newSourcesManager()
			if err != nil {
				return err
			}

			source, err := manager.Get(sourceName)
			if err != nil {
				return err
			}

			projectRoot, err := runtime.FindRepoRoot(".")
			if err != nil {
				return err
			}

			state, err := pinning.Read(projectRoot)
			if err != nil {
				return err
			}

			if _, ok := pinning.Get(state, sourceName); !ok {
				output.Infof("%s is not pinned", sourceName)
				return nil
			}

			remapped, err := remapPinnedMountsToDefault(projectRoot, manager.Paths, source, manager.WorktreePath(source, projectRoot))
			if err != nil {
				return err
			}

			if err := manager.WorktreeRemove(context.Background(), source, projectRoot); err != nil {
				output.Warnf("could not remove worktree: %v", err)
			}

			state = pinning.Remove(state, sourceName)
			if err := pinning.Write(projectRoot, state); err != nil {
				return err
			}

			output.Successf("unpinned %s", sourceName)
			if remapped > 0 {
				output.Infof("updated %d mounted file(s) to the default checkout", remapped)
			}
			return nil
		},
	}
}

func remapPinnedMountsToDefault(projectRoot string, paths runtime.Paths, source sources.Source, worktreePath string) (int, error) {
	state, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return 0, err
	}
	if len(state.Entries) == 0 {
		return 0, nil
	}

	lm := linkmode.Detect()
	defaultRoot := source.CheckoutPath(paths)
	updated := 0

	for index := range state.Entries {
		entry := &state.Entries[index]
		if entry.SourceName != source.Name || !runtime.PathStartsWith(entry.SourcePath, worktreePath) {
			continue
		}

		sourceRel, err := filepath.Rel(worktreePath, entry.SourcePath)
		if err != nil {
			return updated, err
		}
		newSourcePath := filepath.Join(defaultRoot, sourceRel)
		if _, err := os.Stat(newSourcePath); err != nil {
			return updated, fmt.Errorf("cannot update %s after unpin: %w", entry.RelativePath, err)
		}

		targetPath := filepath.Join(projectRoot, filepath.FromSlash(entry.RelativePath))
		switch {
		case entry.Generated:
			// Generated content does not depend on the checkout; only the
			// provenance path moves.
			entry.SourcePath = newSourcePath
		case entry.IsCopy():
			// Copies follow the default checkout only if the user has not
			// edited them; an edited copy is left alone and reported by check.
			if info, err := os.Lstat(targetPath); err == nil && info.Mode().IsRegular() {
				actualHash, err := workspace.FileSHA256(targetPath)
				if err != nil {
					return updated, err
				}
				if actualHash == entry.ContentSHA256 {
					if err := os.Remove(targetPath); err != nil {
						return updated, err
					}
					hash, err := workspace.CopyFile(newSourcePath, targetPath)
					if err != nil {
						return updated, err
					}
					entry.ContentSHA256 = hash
					entry.SourceSHA256 = hash
				}
			} else if err != nil && !os.IsNotExist(err) {
				return updated, err
			}
			entry.SourcePath = newSourcePath
		default:
			if _, err := os.Lstat(targetPath); err == nil {
				if !linkmode.IsLink(lm, targetPath, entry.SourcePath) {
					return updated, fmt.Errorf("cannot update %s after unpin: target is no longer the expected csaw link", entry.RelativePath)
				}
				if err := os.Remove(targetPath); err != nil {
					return updated, err
				}
			} else if !os.IsNotExist(err) {
				return updated, err
			}

			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return updated, err
			}
			if err := linkmode.Create(lm, newSourcePath, targetPath); err != nil {
				return updated, err
			}

			entry.SourcePath = newSourcePath
			if entry.Protected {
				hash, err := workspace.FileSHA256(newSourcePath)
				if err != nil {
					return updated, err
				}
				entry.SourceSHA256 = hash
			}
		}
		updated++
	}

	if updated == 0 {
		return 0, nil
	}

	sort.Slice(state.Entries, func(i, j int) bool {
		return state.Entries[i].RelativePath < state.Entries[j].RelativePath
	})
	if err := workspace.WriteMountState(projectRoot, state); err != nil {
		return updated, err
	}
	if err := workspace.WriteRestoreState(paths, projectRoot, state); err != nil {
		return updated, err
	}
	return updated, nil
}
