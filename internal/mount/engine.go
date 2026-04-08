package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/csaw-ai/csaw/internal/drift"
	"github.com/csaw-ai/csaw/internal/linkmode"
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
	var err error
	entries, err = resolveConflictsByPriority(entries)
	if err != nil {
		return Result{}, err
	}

	lm := linkmode.Detect()

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

		if _, err := os.Lstat(targetPath); err == nil {
			if linkmode.IsLink(lm, targetPath, entry.FullPath) {
				healthy, _ := linkmode.Verify(lm, targetPath, entry.FullPath, runtime.PathsEqual)
				if healthy {
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
		if err := linkmode.Create(lm, entry.FullPath, targetPath); err != nil {
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

	lm := linkmode.Detect()

	store := workspace.FileStateStore{}
	result := Result{}
	var removedPaths []string

	for _, entry := range selected {
		targetPath := filepath.Join(projectRoot, filepath.FromSlash(entry.RelativePath))
		if _, err := os.Lstat(targetPath); err == nil {
			if linkmode.IsLink(lm, targetPath, entry.SourcePath) {
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

	lm := linkmode.Detect()

	statuses := drift.InspectMountState(projectRoot, state, lm)
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
			// Remove existing file/link if present
			if _, err := os.Lstat(targetPath); err == nil {
				if err := os.Remove(targetPath); err != nil {
					return result, statuses, err
				}
			} else if !os.IsNotExist(err) {
				return result, statuses, err
			}

			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return result, statuses, err
			}
			if err := linkmode.Create(lm, status.ExpectedSource, targetPath); err != nil {
				return result, statuses, err
			}
			if _, err := workspace.AddExclusion(projectRoot, status.RelativePath); err != nil {
				return result, statuses, err
			}
			result.Linked++
		}
	}

	return result, drift.InspectMountState(projectRoot, state, lm), nil
}

func resolveConflictsByPriority(entries []SourceEntry) ([]SourceEntry, error) {
	groups := make(map[string][]SourceEntry)
	for _, entry := range entries {
		groups[entry.RelativePath] = append(groups[entry.RelativePath], entry)
	}

	resolved := make([]SourceEntry, 0, len(entries))
	var problems []string

	for _, entry := range entries {
		group := groups[entry.RelativePath]
		if len(group) < 2 {
			resolved = append(resolved, entry)
			continue
		}

		// Find the winner (highest priority)
		best := group[0]
		tied := false
		for _, candidate := range group[1:] {
			if candidate.Priority > best.Priority {
				best = candidate
				tied = false
			} else if candidate.Priority == best.Priority {
				tied = true
			}
		}

		if tied {
			names := make([]string, len(group))
			for i, g := range group {
				names[i] = g.SourceName
			}
			sort.Strings(names)
			problems = append(problems, fmt.Sprintf("%s (%s)", entry.RelativePath, strings.Join(names, ", ")))
			continue
		}

		// Only add the winner once
		if entry.SourceName == best.SourceName {
			resolved = append(resolved, best)
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		// Deduplicate problem strings (each conflict is reported once per group member)
		deduped := problems[:0]
		seen := make(map[string]bool)
		for _, p := range problems {
			if !seen[p] {
				seen[p] = true
				deduped = append(deduped, p)
			}
		}
		return nil, fmt.Errorf("ambiguous mount selection; multiple sources with equal priority provide the same target path: %s\nUse source priority to resolve: csaw source add <name> <url> --priority <n>", strings.Join(deduped, "; "))
	}

	return resolved, nil
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
