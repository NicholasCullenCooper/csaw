package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/csaw-ai/csaw/internal/drift"
	"github.com/csaw-ai/csaw/internal/runtime"
	"github.com/csaw-ai/csaw/internal/workspace"
)

type ConflictAction string

const (
	ConflictOverwrite ConflictAction = "overwrite"
	ConflictSkip      ConflictAction = "skip"
)

type Conflict struct {
	RelativePath string
	SourceName   string
	SourcePath   string
	TargetPath   string
}

type ConflictResolver interface {
	Resolve(Conflict) (ConflictAction, error)
}

type Result struct {
	Linked        int
	Stashed       int
	Skipped       int
	AlreadyLinked int
	Removed       int
	Restored      int
}

func Apply(projectRoot string, paths runtime.Paths, entries []SourceEntry, resolver ConflictResolver) (Result, error) {
	if err := ensureUniqueTargets(entries); err != nil {
		return Result{}, err
	}

	store := workspace.FileStateStore{}
	currentState, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	for _, entry := range entries {
		targetPath := filepath.Join(projectRoot, filepath.FromSlash(entry.RelativePath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return result, err
		}

		if info, err := os.Lstat(targetPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				linkTarget, err := os.Readlink(targetPath)
				if err != nil {
					return result, err
				}
				resolved := linkTarget
				if !filepath.IsAbs(resolved) {
					resolved = filepath.Join(filepath.Dir(targetPath), resolved)
				}
				if runtime.PathsEqual(resolved, entry.FullPath) {
					currentState = workspace.UpsertMountedEntry(currentState, toMountedStateEntry(entry))
					if !workspace.IsGitIgnored(projectRoot, entry.RelativePath) {
						if _, err := workspace.AddExclusion(projectRoot, entry.RelativePath); err != nil {
							return result, err
						}
					}
					result.AlreadyLinked++
					continue
				}
			}

			action, err := resolver.Resolve(Conflict{
				RelativePath: entry.RelativePath,
				SourceName:   entry.SourceName,
				SourcePath:   entry.FullPath,
				TargetPath:   targetPath,
			})
			if err != nil {
				return result, err
			}
			if action == ConflictSkip {
				result.Skipped++
				continue
			}

			stat, err := os.Stat(targetPath)
			if err == nil && stat.IsDir() {
				return result, fmt.Errorf("cannot overwrite directory target for %s", entry.RelativePath)
			}

			if err := workspace.StashFile(store, projectRoot, entry.RelativePath, entry.FullPath); err != nil {
				return result, err
			}
			result.Stashed++

			if err := os.Remove(targetPath); err != nil {
				return result, err
			}
		} else if !os.IsNotExist(err) {
			return result, err
		}

		if _, err := os.Stat(entry.FullPath); err != nil {
			return result, err
		}
		if err := os.Symlink(entry.FullPath, targetPath); err != nil {
			return result, err
		}

		// Only add git exclude if the path isn't already covered by .gitignore
		if !workspace.IsGitIgnored(projectRoot, entry.RelativePath) {
			if _, err := workspace.AddExclusion(projectRoot, entry.RelativePath); err != nil {
				return result, err
			}
		}

		currentState = workspace.UpsertMountedEntry(currentState, toMountedStateEntry(entry))
		result.Linked++
	}

	if len(currentState.Entries) > 0 {
		sort.Slice(currentState.Entries, func(i, j int) bool {
			return currentState.Entries[i].RelativePath < currentState.Entries[j].RelativePath
		})
		if err := workspace.WriteMountState(projectRoot, currentState); err != nil {
			return result, err
		}
		if err := workspace.WriteRestoreState(paths, projectRoot, currentState); err != nil {
			return result, err
		}
		if _, err := workspace.AddExclusion(projectRoot, runtime.StashDirName); err != nil {
			return result, err
		}
	}

	return result, nil
}

func Unmount(projectRoot string, selection Selection) (Result, error) {
	state, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return Result{}, err
	}

	selected, err := filterMountedStateEntries(state.Entries, selection)
	if err != nil {
		return Result{}, err
	}

	store := workspace.FileStateStore{}
	result := Result{}
	var removedPaths []string

	for _, entry := range selected {
		targetPath := filepath.Join(projectRoot, filepath.FromSlash(entry.RelativePath))
		if info, err := os.Lstat(targetPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(targetPath); err != nil {
					return result, err
				}
				result.Removed++
			}
		} else if !os.IsNotExist(err) {
			return result, err
		}

		restored, err := workspace.RestoreFile(store, projectRoot, entry.RelativePath)
		if err != nil {
			return result, err
		}
		if restored {
			result.Restored++
		}

		if _, err := workspace.RemoveExclusion(projectRoot, entry.RelativePath); err != nil {
			return result, err
		}
		removedPaths = append(removedPaths, entry.RelativePath)
	}

	state = workspace.RemoveMountedEntries(state, removedPaths)
	if err := workspace.WriteMountState(projectRoot, state); err != nil {
		return result, err
	}
	if err := workspace.CleanupStash(store, projectRoot); err != nil {
		return result, err
	}
	if _, err := os.Stat(workspace.StashDir(projectRoot)); os.IsNotExist(err) {
		if _, err := workspace.RemoveExclusion(projectRoot, runtime.StashDirName); err != nil {
			return result, err
		}
	}

	cleanupEmptyDirs(projectRoot, removedPaths)

	return result, nil
}

// cleanupEmptyDirs removes empty directories left behind after symlink removal.
// It collects all ancestor directories, sorts deepest-first, and removes each
// one that is empty. It never removes projectRoot itself.
func cleanupEmptyDirs(projectRoot string, removedPaths []string) {
	// Collect all ancestor directories of removed files
	candidates := make(map[string]bool)
	for _, relPath := range removedPaths {
		dir := filepath.Dir(filepath.FromSlash(relPath))
		for dir != "." && dir != "" {
			candidates[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	// Sort deepest first so children are removed before parents
	sorted := make([]string, 0, len(candidates))
	for dir := range candidates {
		sorted = append(sorted, dir)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	for _, dir := range sorted {
		absDir := filepath.Join(projectRoot, dir)
		entries, err := os.ReadDir(absDir)
		if err != nil || len(entries) > 0 {
			continue
		}
		os.Remove(absDir)
	}
}

func EntriesFromMountedState(state workspace.MountState) []SourceEntry {
	entries := make([]SourceEntry, 0, len(state.Entries))
	for _, entry := range state.Entries {
		entries = append(entries, SourceEntry{
			SourceName:    entry.SourceName,
			RelativePath:  entry.RelativePath,
			QualifiedPath: entry.SourceName + "/" + entry.RelativePath,
			FullPath:      entry.SourcePath,
		})
	}
	return entries
}

func Repair(projectRoot string) (Result, []drift.Status, error) {
	state, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return Result{}, nil, err
	}

	statuses := drift.InspectMountState(projectRoot, state)
	result := Result{}

	for _, status := range statuses {
		if status.Healthy {
			continue
		}

		switch status.Issue {
		case drift.IssueMissingSource, drift.IssueReplacedLink:
			continue
		case drift.IssueDriftedLink, drift.IssueMissingLink:
			targetPath := filepath.Join(projectRoot, filepath.FromSlash(status.RelativePath))
			if info, err := os.Lstat(targetPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(targetPath); err != nil {
					return result, statuses, err
				}
			} else if err != nil && !os.IsNotExist(err) {
				return result, statuses, err
			}

			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return result, statuses, err
			}
			if err := os.Symlink(status.ExpectedSource, targetPath); err != nil {
				return result, statuses, err
			}
			if _, err := workspace.AddExclusion(projectRoot, status.RelativePath); err != nil {
				return result, statuses, err
			}
			result.Linked++
		}
	}

	return result, drift.InspectMountState(projectRoot, state), nil
}

func ensureUniqueTargets(entries []SourceEntry) error {
	conflicts := make(map[string][]string)
	for _, entry := range entries {
		conflicts[entry.RelativePath] = append(conflicts[entry.RelativePath], entry.SourceName)
	}

	var problems []string
	for path, sourceNames := range conflicts {
		if len(sourceNames) < 2 {
			continue
		}
		sort.Strings(sourceNames)
		problems = append(problems, fmt.Sprintf("%s (%s)", path, strings.Join(sourceNames, ", ")))
	}

	if len(problems) == 0 {
		return nil
	}

	sort.Strings(problems)
	return fmt.Errorf("ambiguous mount selection; multiple sources provide the same target path: %s", strings.Join(problems, "; "))
}

func filterMountedStateEntries(entries []workspace.MountedStateEntry, selection Selection) ([]workspace.MountedStateEntry, error) {
	if selection.IsEmpty() {
		return append([]workspace.MountedStateEntry(nil), entries...), nil
	}

	filtered := make([]workspace.MountedStateEntry, 0, len(entries))
	for _, entry := range entries {
		sourceEntry := SourceEntry{
			SourceName:    entry.SourceName,
			RelativePath:  entry.RelativePath,
			QualifiedPath: entry.SourceName + "/" + entry.RelativePath,
			FullPath:      entry.SourcePath,
		}
		include, err := matchesQualified(sourceEntry, selection.IncludePatterns, true)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		exclude, err := matchesQualified(sourceEntry, selection.ExcludePatterns, false)
		if err != nil {
			return nil, err
		}
		if exclude {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, nil
}

func toMountedStateEntry(entry SourceEntry) workspace.MountedStateEntry {
	return workspace.MountedStateEntry{
		RelativePath: entry.RelativePath,
		SourceName:   entry.SourceName,
		SourcePath:   entry.FullPath,
	}
}
