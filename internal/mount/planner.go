package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
)

type Selection struct {
	IncludePatterns []string
	ExcludePatterns []string
	Profile         string
	IncludeIgnored  bool
}

type Planner interface {
	Filter(entries []string, selection Selection) ([]string, error)
}

type GlobPlanner struct{}

type SourceEntry struct {
	SourceName    string
	RelativePath  string
	QualifiedPath string
	FullPath      string
	Priority      int
}

func NewPlanner() Planner {
	return GlobPlanner{}
}

func (selection Selection) IsEmpty() bool {
	return len(selection.IncludePatterns) == 0 && len(selection.ExcludePatterns) == 0 && selection.Profile == "" && !selection.IncludeIgnored
}

func (selection Selection) String() string {
	parts := make([]string, 0, 3)
	if selection.Profile != "" {
		parts = append(parts, "profile="+selection.Profile)
	}
	if len(selection.IncludePatterns) > 0 {
		parts = append(parts, "include="+strings.Join(selection.IncludePatterns, ","))
	}
	if len(selection.ExcludePatterns) > 0 {
		parts = append(parts, "exclude="+strings.Join(selection.ExcludePatterns, ","))
	}
	if selection.IncludeIgnored {
		parts = append(parts, "includeIgnored=true")
	}
	if len(parts) == 0 {
		return "default"
	}
	return strings.Join(parts, " ")
}

func (GlobPlanner) Filter(entries []string, selection Selection) ([]string, error) {
	filtered := make([]string, 0, len(entries))

	for _, entry := range entries {
		normalized := runtime.NormalizeRegistryPath(entry)
		include, err := matchesAny(normalized, selection.IncludePatterns, true)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}

		exclude, err := matchesAny(normalized, selection.ExcludePatterns, false)
		if err != nil {
			return nil, err
		}
		if exclude {
			continue
		}

		filtered = append(filtered, normalized)
	}

	sort.Strings(filtered)
	return filtered, nil
}

func matchesAny(entry string, patterns []string, defaultValue bool) (bool, error) {
	if len(patterns) == 0 {
		return defaultValue, nil
	}

	for _, pattern := range patterns {
		match, err := matchPattern(entry, pattern)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}

	return false, nil
}

func matchPattern(entry string, pattern string) (bool, error) {
	normalized := runtime.NormalizeRegistryPath(pattern)
	if normalized == "" {
		return false, fmt.Errorf("invalid empty pattern")
	}

	if !hasGlob(normalized) {
		return entry == normalized || strings.HasPrefix(entry, normalized+"/"), nil
	}

	return doublestar.PathMatch(normalized, entry)
}

func hasGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func EnumerateSourceEntries(source sources.CatalogSource) ([]SourceEntry, error) {
	var entries []SourceEntry

	if _, err := os.Stat(source.Root); err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}

	err := filepath.WalkDir(source.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()
		if d.IsDir() && (name == ".git" || runtime.IsNoiseFile(name)) {
			if name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		if runtime.IsNoiseFile(name) {
			return nil
		}

		relativePath, err := filepath.Rel(source.Root, path)
		if err != nil {
			return err
		}
		relativePath = runtime.NormalizeRegistryPath(relativePath)
		if relativePath == runtime.ProfilesFile || relativePath == runtime.IgnoreFile {
			return nil
		}

		entries = append(entries, SourceEntry{
			SourceName:    source.Name,
			RelativePath:  relativePath,
			QualifiedPath: source.Name + "/" + relativePath,
			FullPath:      path,
			Priority:      source.Priority,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].QualifiedPath == entries[j].QualifiedPath {
			return entries[i].FullPath < entries[j].FullPath
		}
		return entries[i].QualifiedPath < entries[j].QualifiedPath
	})
	return entries, nil
}

func ReadIgnorePatterns(root string) ([]string, error) {
	ignorePath := filepath.Join(root, runtime.IgnoreFile)
	content, err := os.ReadFile(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var patterns []string
	for _, line := range strings.Split(runtime.StripBOM(string(content)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, nil
}

func ApplyIgnore(entries []SourceEntry, patterns []string) ([]SourceEntry, error) {
	if len(patterns) == 0 {
		return entries, nil
	}

	filtered := make([]SourceEntry, 0, len(entries))
	for _, entry := range entries {
		excluded, err := matchesAny(entry.RelativePath, patterns, false)
		if err != nil {
			return nil, err
		}
		if excluded {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, nil
}

func FilterSourceEntries(entries []SourceEntry, selection Selection) ([]SourceEntry, error) {
	filtered := make([]SourceEntry, 0, len(entries))

	for _, entry := range entries {
		include, err := matchesQualified(entry, selection.IncludePatterns, true)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}

		exclude, err := matchesQualified(entry, selection.ExcludePatterns, false)
		if err != nil {
			return nil, err
		}
		if exclude {
			continue
		}

		filtered = append(filtered, entry)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].RelativePath == filtered[j].RelativePath {
			return filtered[i].QualifiedPath < filtered[j].QualifiedPath
		}
		return filtered[i].RelativePath < filtered[j].RelativePath
	})
	return filtered, nil
}

func matchesQualified(entry SourceEntry, patterns []string, defaultValue bool) (bool, error) {
	if len(patterns) == 0 {
		return defaultValue, nil
	}

	for _, pattern := range patterns {
		matched, err := matchesPattern(entry, pattern)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
}

func matchesPattern(entry SourceEntry, pattern string) (bool, error) {
	normalized := runtime.NormalizeRegistryPath(pattern)
	if normalized == "" {
		return false, fmt.Errorf("invalid empty pattern")
	}

	for _, candidate := range []string{entry.QualifiedPath, entry.RelativePath} {
		match, err := matchPattern(candidate, normalized)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}

	return false, nil
}
