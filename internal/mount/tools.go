package mount

import (
	"os"
	"path/filepath"
	"strings"
)

// ToolDir describes a tool's skill directory convention.
type ToolDir struct {
	// Dir is the dot-directory name (e.g., ".claude").
	Dir string
	// SkillsSubdir is the path under Dir where skills are stored (e.g., "skills").
	SkillsSubdir string
}

// KnownToolDirs lists the tool directories csaw auto-detects. Each tool that
// supports the SKILL.md standard gets an entry here.
var KnownToolDirs = []ToolDir{
	{Dir: ".claude", SkillsSubdir: "skills"},
	{Dir: ".opencode", SkillsSubdir: "skills"},
	{Dir: ".agents", SkillsSubdir: "skills"},
	{Dir: ".codex", SkillsSubdir: "skills"},
}

// DetectToolDirs returns tool directories that exist in the project root.
func DetectToolDirs(projectRoot string) []ToolDir {
	var found []ToolDir
	for _, tool := range KnownToolDirs {
		dir := filepath.Join(projectRoot, tool.Dir)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			found = append(found, tool)
		}
	}
	return found
}

// isSkillEntry returns true if the source entry looks like a skill
// (lives under a skills/ directory and is named SKILL.md).
func isSkillEntry(entry SourceEntry) bool {
	rel := entry.RelativePath
	return strings.HasSuffix(rel, "/SKILL.md") && containsSegment(rel, "skills")
}

// skillName extracts the skill directory name from a skill entry path.
// e.g., "skills/code-review/SKILL.md" → "code-review"
func skillName(entry SourceEntry) string {
	dir := filepath.Dir(entry.RelativePath)
	return filepath.Base(dir)
}

// ExpandToolTargets takes a list of source entries and expands skill entries
// into additional mount targets for each detected tool directory. Non-skill
// entries (AGENTS.md, CLAUDE.md, etc.) are left at their original paths.
//
// The original entry is kept as-is (mounted at its registry-relative path).
// Additional entries are created for each tool directory.
func ExpandToolTargets(entries []SourceEntry, toolDirs []ToolDir) []SourceEntry {
	if len(toolDirs) == 0 {
		return entries
	}

	var expanded []SourceEntry
	for _, entry := range entries {
		// Always keep the original entry
		expanded = append(expanded, entry)

		if !isSkillEntry(entry) {
			continue
		}

		name := skillName(entry)
		for _, tool := range toolDirs {
			toolRelPath := filepath.ToSlash(
				filepath.Join(tool.Dir, tool.SkillsSubdir, name, "SKILL.md"),
			)
			// Don't duplicate if the original path already matches this tool dir
			if toolRelPath == entry.RelativePath {
				continue
			}
			expanded = append(expanded, SourceEntry{
				SourceName:    entry.SourceName,
				RelativePath:  toolRelPath,
				QualifiedPath: entry.QualifiedPath + "→" + toolRelPath,
				FullPath:      entry.FullPath,
			})
		}
	}

	return expanded
}

func containsSegment(path string, segment string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}
