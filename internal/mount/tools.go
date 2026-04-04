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

// StandardFallback is always used as a skill mount target, created if needed.
var StandardFallback = ToolDir{Dir: ".agents", SkillsSubdir: "skills"}

// DetectToolDirs returns tool directories to mount skills into. It detects
// which known tool directories already exist, and always includes .agents/
// as the standard fallback (creating it if needed).
func DetectToolDirs(projectRoot string) []ToolDir {
	found := make(map[string]bool)
	var dirs []ToolDir

	for _, tool := range KnownToolDirs {
		dir := filepath.Join(projectRoot, tool.Dir)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			found[tool.Dir] = true
			dirs = append(dirs, tool)
		}
	}

	// Always include the standard fallback
	if !found[StandardFallback.Dir] {
		fallbackPath := filepath.Join(projectRoot, StandardFallback.Dir)
		os.MkdirAll(fallbackPath, 0o755)
		dirs = append(dirs, StandardFallback)
	}

	return dirs
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

// ExpandToolTargets takes a list of source entries and redirects skill entries
// into tool-specific directories. Non-skill entries (AGENTS.md, CLAUDE.md,
// agents/, commands/, workflows/) are kept at their original paths.
//
// Skill entries are NOT mounted at their original registry path (e.g.,
// skills/code-review/SKILL.md). Instead, they are mounted only into tool
// directories (e.g., .claude/skills/code-review/SKILL.md). This ensures
// skills are discovered by tool-native scanning rather than relying on
// git-aware file indexing.
func ExpandToolTargets(entries []SourceEntry, toolDirs []ToolDir) []SourceEntry {
	// First pass: project MCP configs to tool-specific paths.
	entries = expandMCPTargets(entries)

	var expanded []SourceEntry
	for _, entry := range entries {
		if !isSkillEntry(entry) {
			// Non-skill: keep at original path
			expanded = append(expanded, entry)
			continue
		}

		if len(toolDirs) == 0 {
			// No tool dirs at all: fall back to original path
			expanded = append(expanded, entry)
			continue
		}

		// Skill: mount only into tool directories, not at original path
		name := skillName(entry)
		for _, tool := range toolDirs {
			toolRelPath := filepath.ToSlash(
				filepath.Join(tool.Dir, tool.SkillsSubdir, name, "SKILL.md"),
			)
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

// MCPTarget maps a registry filename under mcp/ to a project-relative path
// where the corresponding tool expects its MCP config.
type MCPTarget struct {
	// RegistryFile is the filename in the mcp/ directory (e.g., "claude-code.json").
	RegistryFile string
	// ProjectPath is the relative path in the project (e.g., ".mcp.json").
	ProjectPath string
}

// KnownMCPTargets lists the supported MCP config projections. Each entry maps
// a file in the registry's mcp/ directory to the path a tool reads from.
var KnownMCPTargets = []MCPTarget{
	{RegistryFile: "claude-code.json", ProjectPath: ".mcp.json"},
	{RegistryFile: "vscode.json", ProjectPath: ".vscode/mcp.json"},
	{RegistryFile: "cursor.json", ProjectPath: ".cursor/mcp.json"},
}

// isMCPEntry returns true if the source entry is an MCP config file
// (lives directly under the mcp/ directory and is a .json file).
func isMCPEntry(entry SourceEntry) bool {
	rel := entry.RelativePath
	dir := filepath.Dir(rel)
	return dir == "mcp" && strings.HasSuffix(rel, ".json")
}

// mcpProjectPath returns the project-relative target path for an MCP entry,
// or empty string if the filename is not a known target.
func mcpProjectPath(entry SourceEntry) string {
	base := filepath.Base(entry.RelativePath)
	for _, target := range KnownMCPTargets {
		if base == target.RegistryFile {
			return target.ProjectPath
		}
	}
	return ""
}

// expandMCPTargets redirects MCP config entries from their registry paths
// (mcp/claude-code.json) to tool-specific project paths (.mcp.json). Unknown
// MCP files are kept at their original path.
func expandMCPTargets(entries []SourceEntry) []SourceEntry {
	var expanded []SourceEntry
	for _, entry := range entries {
		if !isMCPEntry(entry) {
			expanded = append(expanded, entry)
			continue
		}
		projectPath := mcpProjectPath(entry)
		if projectPath == "" {
			// Unknown MCP file: keep at original path
			expanded = append(expanded, entry)
			continue
		}
		expanded = append(expanded, SourceEntry{
			SourceName:    entry.SourceName,
			RelativePath:  projectPath,
			QualifiedPath: entry.QualifiedPath + "→" + projectPath,
			FullPath:      entry.FullPath,
		})
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
