package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NicholasCullenCooper/csaw/internal/drift"
	"github.com/NicholasCullenCooper/csaw/internal/hooks"
	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
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
	// Linked counts files placed (copied, written, or linked).
	Linked        int
	Stashed       int
	Skipped       int
	AlreadyLinked int
	Removed       int
	Restored      int
	// KeptModified lists files a user edited after csaw placed them; unmount
	// preserves them under .csaw-stash/modified/ instead of deleting the edits.
	KeptModified []string
}

// Apply places entries using the default mount mode (copy).
func Apply(projectRoot string, paths runtime.Paths, entries []SourceEntry, resolver ConflictResolver) (Result, error) {
	return ApplyWithOptions(projectRoot, paths, entries, resolver, ApplyOptions{Mode: DefaultMountMode})
}

// ApplyWithOptions places entries in the project as copies or links,
// stashing displaced originals, recording mount state, and hiding paths
// from git.
func ApplyWithOptions(projectRoot string, paths runtime.Paths, entries []SourceEntry, resolver ConflictResolver, options ApplyOptions) (Result, error) {
	var err error
	entries, err = resolveConflictsByPriority(entries)
	if err != nil {
		return Result{}, err
	}

	mode := options.Mode
	if mode == "" {
		mode = DefaultMountMode
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

		if entry.MergeHooks {
			if entry.Content == "" {
				continue // nothing this tool can run; notes explain why
			}
			stateEntry, merged, err := mergeHooksEntry(projectRoot, targetPath, entry)
			if err != nil {
				return result, err
			}
			currentState = workspace.UpsertMountedEntry(currentState, stateEntry)
			if merged {
				result.Linked++
			} else {
				result.AlreadyLinked++
			}
			continue
		}

		if _, err := os.Lstat(targetPath); err == nil {
			if alreadyMounted(lm, mode, targetPath, entry) {
				stateEntry, err := toMountedStateEntry(entry, mode)
				if err != nil {
					return result, err
				}
				currentState = workspace.UpsertMountedEntry(currentState, stateEntry)
				if !workspace.IsGitIgnored(projectRoot, entry.RelativePath) {
					if _, err := workspace.AddExclusion(projectRoot, entry.RelativePath); err != nil {
						return result, err
					}
				}
				result.AlreadyLinked++
				continue
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

		if err := placeEntry(lm, mode, entry, targetPath); err != nil {
			return result, err
		}

		// Only add git exclude if the path isn't already covered by .gitignore
		if !workspace.IsGitIgnored(projectRoot, entry.RelativePath) {
			if _, err := workspace.AddExclusion(projectRoot, entry.RelativePath); err != nil {
				return result, err
			}
		}

		stateEntry, err := toMountedStateEntry(entry, mode)
		if err != nil {
			return result, err
		}
		currentState = workspace.UpsertMountedEntry(currentState, stateEntry)
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

// mergeHooksEntry merges an owned hooks fragment into the tool's hooks file.
// The target existing is normal, not a conflict: user groups stay, csaw's
// are appended. Git exclusion is added only for a file csaw created.
func mergeHooksEntry(projectRoot string, targetPath string, entry SourceEntry) (workspace.MountedStateEntry, bool, error) {
	owned, err := hooks.Decode(entry.Content)
	if err != nil {
		return workspace.MountedStateEntry{}, false, err
	}
	stateEntry := workspace.MountedStateEntry{
		RelativePath:  entry.RelativePath,
		SourceName:    entry.SourceName,
		SourcePath:    entry.FullPath,
		Protected:     entry.Protected,
		Mode:          workspace.ModeMergeHooks,
		Content:       entry.Content,
		ContentSHA256: workspace.ContentSHA256([]byte(entry.Content)),
	}
	if _, missing, err := hooks.Inspect(targetPath, owned); err == nil && missing == 0 {
		return stateEntry, false, nil
	}
	created, err := hooks.Merge(targetPath, owned)
	if err != nil {
		return workspace.MountedStateEntry{}, false, err
	}
	if created {
		stateEntry.CreatedFile = true
		if !workspace.IsGitIgnored(projectRoot, entry.RelativePath) {
			if _, err := workspace.AddExclusion(projectRoot, entry.RelativePath); err != nil {
				return workspace.MountedStateEntry{}, false, err
			}
		}
	}
	return stateEntry, true, nil
}

// placeEntry writes a generated file, copies the source, or creates a link,
// depending on the entry and mode.
func placeEntry(lm linkmode.Mode, mode MountMode, entry SourceEntry, targetPath string) error {
	if entry.Generated {
		return os.WriteFile(targetPath, []byte(entry.Content), 0o644)
	}
	if _, err := os.Stat(entry.FullPath); err != nil {
		return err
	}
	if mode == ModeCopy {
		_, err := workspace.CopyFile(entry.FullPath, targetPath)
		return err
	}
	return linkmode.Create(lm, entry.FullPath, targetPath)
}

// alreadyMounted reports whether targetPath already holds what entry would
// produce: the same bytes for generated and copied entries, or the expected
// link for linked ones.
func alreadyMounted(lm linkmode.Mode, mode MountMode, targetPath string, entry SourceEntry) bool {
	if entry.Generated || mode == ModeCopy {
		info, err := os.Lstat(targetPath)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
		actualHash, err := workspace.FileSHA256(targetPath)
		if err != nil {
			return false
		}
		if entry.Generated {
			return actualHash == workspace.ContentSHA256([]byte(entry.Content))
		}
		sourceHash, err := workspace.FileSHA256(entry.FullPath)
		return err == nil && actualHash == sourceHash
	}
	if !linkmode.IsLink(lm, targetPath, entry.FullPath) {
		return false
	}
	healthy, _ := linkmode.Verify(lm, targetPath, entry.FullPath, runtime.PathsEqual)
	return healthy
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
		if entry.IsMerge() {
			owned, err := hooks.Decode(entry.Content)
			if err != nil {
				return result, err
			}
			deleted, err := hooks.Remove(targetPath, owned)
			if err != nil {
				return result, err
			}
			result.Removed++
			if deleted || entry.CreatedFile {
				if _, err := workspace.RemoveExclusion(projectRoot, entry.RelativePath); err != nil {
					return result, err
				}
			}
			removedPaths = append(removedPaths, entry.RelativePath)
			continue
		}
		if info, err := os.Lstat(targetPath); err == nil {
			switch {
			case entry.IsCopy() && info.Mode().IsRegular():
				actualHash, err := workspace.FileSHA256(targetPath)
				if err != nil {
					return result, err
				}
				if actualHash == entry.ContentSHA256 {
					if err := os.Remove(targetPath); err != nil {
						return result, err
					}
				} else {
					kept, err := workspace.StashModified(projectRoot, entry.RelativePath)
					if err != nil {
						return result, err
					}
					result.KeptModified = append(result.KeptModified, kept)
				}
				result.Removed++
			case !entry.IsCopy() && linkmode.IsLink(lm, targetPath, entry.SourcePath):
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

// cleanupEmptyDirs removes empty directories left behind after file removal.
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
			Protected:     entry.Protected,
			Generated:     entry.Generated,
			Content:       entry.Content,
			MergeHooks:    entry.IsMerge(),
		})
	}
	return entries
}

// Repair brings unhealthy entries back without discarding anything:
// missing or stale copies are rewritten from the source, missing or drifted
// links are recreated, missing generated files are rewritten. Locally
// modified files are left alone and reported in the returned statuses.
func Repair(projectRoot string) (Result, []drift.Status, error) {
	state, err := workspace.ReadMountState(projectRoot)
	if err != nil {
		return Result{}, nil, err
	}

	lm := linkmode.Detect()

	statuses := drift.InspectMountState(projectRoot, state, lm)
	result := Result{}

	entryIndex := make(map[string]int, len(state.Entries))
	for i, entry := range state.Entries {
		entryIndex[entry.RelativePath] = i
	}
	stateChanged := false

	for _, status := range statuses {
		if status.Healthy || !status.NeedsUpdate() {
			continue
		}
		index, ok := entryIndex[status.RelativePath]
		if !ok {
			continue
		}
		entry := &state.Entries[index]
		targetPath := filepath.Join(projectRoot, filepath.FromSlash(entry.RelativePath))

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return result, statuses, err
		}

		switch {
		case entry.IsMerge():
			owned, err := hooks.Decode(entry.Content)
			if err != nil {
				return result, statuses, err
			}
			created, err := hooks.Merge(targetPath, owned)
			if err != nil {
				return result, statuses, err
			}
			if created {
				entry.CreatedFile = true
				stateChanged = true
			}
			result.Linked++
			continue
		case entry.Generated:
			if err := os.WriteFile(targetPath, []byte(entry.Content), 0o644); err != nil {
				return result, statuses, err
			}
		case entry.Mode == "copy":
			// Remove a stale copy first so the rename cannot race a reader.
			if _, err := os.Lstat(targetPath); err == nil {
				if err := os.Remove(targetPath); err != nil {
					return result, statuses, err
				}
			}
			hash, err := workspace.CopyFile(entry.SourcePath, targetPath)
			if err != nil {
				return result, statuses, err
			}
			entry.ContentSHA256 = hash
			entry.SourceSHA256 = hash
			stateChanged = true
		default:
			if _, err := os.Lstat(targetPath); err == nil {
				if err := os.Remove(targetPath); err != nil {
					return result, statuses, err
				}
			} else if !os.IsNotExist(err) {
				return result, statuses, err
			}
			if err := linkmode.Create(lm, entry.SourcePath, targetPath); err != nil {
				return result, statuses, err
			}
		}
		if _, err := workspace.AddExclusion(projectRoot, status.RelativePath); err != nil {
			return result, statuses, err
		}
		result.Linked++
	}

	if stateChanged {
		if err := workspace.WriteMountState(projectRoot, state); err != nil {
			return result, statuses, err
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
	var protectionViolations []string

	for _, entry := range entries {
		group := groups[entry.RelativePath]
		if len(group) < 2 {
			resolved = append(resolved, entry)
			continue
		}

		// If any entry in the group is protected, the protected one always wins
		// (regardless of priority). Protection overrides priority.
		var protected *SourceEntry
		for i := range group {
			if group[i].Protected {
				if protected != nil && protected.SourceName != group[i].SourceName {
					// Two different sources both protect the same path — hard error
					protectionViolations = append(protectionViolations,
						fmt.Sprintf("%s (protected by both %s and %s)",
							entry.RelativePath, protected.SourceName, group[i].SourceName))
				}
				protected = &group[i]
			}
		}
		if protected != nil {
			if entry.SourceName == protected.SourceName {
				resolved = append(resolved, *protected)
			}
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

	if len(protectionViolations) > 0 {
		sort.Strings(protectionViolations)
		deduped := dedupeStrings(protectionViolations)
		return nil, fmt.Errorf("protection conflict; multiple sources protect the same path: %s", strings.Join(deduped, "; "))
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		deduped := dedupeStrings(problems)
		return nil, fmt.Errorf("ambiguous mount selection; multiple sources with equal priority provide the same target path: %s\nUse source priority to resolve: csaw source add <name> <url> --priority <n>", strings.Join(deduped, "; "))
	}

	return resolved, nil
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool)
	out := values[:0]
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
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
			Protected:     entry.Protected,
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

// toMountedStateEntry records how an entry was placed. Copy entries carry the
// hash of the bytes written (equal to the source hash at mount time);
// symlink entries carry a source hash only when protected.
func toMountedStateEntry(entry SourceEntry, mode MountMode) (workspace.MountedStateEntry, error) {
	stateEntry := workspace.MountedStateEntry{
		RelativePath: entry.RelativePath,
		SourceName:   entry.SourceName,
		SourcePath:   entry.FullPath,
		Protected:    entry.Protected,
	}
	if entry.MergeHooks {
		stateEntry.Mode = workspace.ModeMergeHooks
		stateEntry.Content = entry.Content
		stateEntry.ContentSHA256 = workspace.ContentSHA256([]byte(entry.Content))
		return stateEntry, nil
	}
	if entry.Generated {
		stateEntry.Generated = true
		stateEntry.Content = entry.Content
		stateEntry.ContentSHA256 = workspace.ContentSHA256([]byte(entry.Content))
		return stateEntry, nil
	}
	if mode == ModeCopy {
		hash, err := workspace.FileSHA256(entry.FullPath)
		if err != nil {
			return workspace.MountedStateEntry{}, err
		}
		stateEntry.Mode = string(ModeCopy)
		stateEntry.ContentSHA256 = hash
		stateEntry.SourceSHA256 = hash
		return stateEntry, nil
	}
	stateEntry.Mode = string(ModeSymlink)
	if !entry.Protected {
		return stateEntry, nil
	}

	hash, err := workspace.FileSHA256(entry.FullPath)
	if err != nil {
		return workspace.MountedStateEntry{}, err
	}
	stateEntry.SourceSHA256 = hash
	return stateEntry, nil
}

// PlannedState describes what ApplyWithOptions would record for entries,
// without touching the project. Used to compare a planned mount against the
// lock before anything changes.
func PlannedState(entries []SourceEntry, mode MountMode) (workspace.MountState, error) {
	resolved, err := resolveConflictsByPriority(entries)
	if err != nil {
		return workspace.MountState{}, err
	}
	if mode == "" {
		mode = DefaultMountMode
	}
	var state workspace.MountState
	for _, entry := range resolved {
		if entry.MergeHooks && entry.Content == "" {
			continue
		}
		stateEntry, err := toMountedStateEntry(entry, mode)
		if err != nil {
			return workspace.MountState{}, err
		}
		state = workspace.UpsertMountedEntry(state, stateEntry)
	}
	sort.Slice(state.Entries, func(i, j int) bool {
		return state.Entries[i].RelativePath < state.Entries[j].RelativePath
	})
	return state, nil
}
