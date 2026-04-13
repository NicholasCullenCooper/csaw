package mount

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ToolDir describes a tool's directory conventions for skills, rules, and agents.
type ToolDir struct {
	// Dir is the dot-directory name (e.g., ".claude").
	Dir string
	// SkillsSubdir is the path under Dir where skills are stored (e.g., "skills").
	SkillsSubdir string
	// RulesSubdir is the path under Dir where rule/instruction files are stored.
	// Empty means this tool doesn't have a rules directory.
	RulesSubdir string
	// AgentsSubdir is the path under Dir where subagent definitions are stored.
	// Empty means this tool doesn't support subagents.
	AgentsSubdir string
}

// ToolRegistry maps short tool names to their directory conventions.
var ToolRegistry = map[string]ToolDir{
	"claude":   {Dir: ".claude", SkillsSubdir: "skills", RulesSubdir: "rules", AgentsSubdir: "agents"},
	"opencode": {Dir: ".opencode", SkillsSubdir: "skills"},
	"codex":    {Dir: ".codex", SkillsSubdir: "skills", AgentsSubdir: "agents"},
	"cursor":   {Dir: ".cursor", RulesSubdir: "rules", AgentsSubdir: "agents"},
	"windsurf": {Dir: ".windsurf", RulesSubdir: "rules"},
}

// KnownToolDirs returns all known tool directories.
func KnownToolDirs() []ToolDir {
	var dirs []ToolDir
	for _, tool := range ToolRegistry {
		dirs = append(dirs, tool)
	}
	return dirs
}

// AllToolNames returns all known tool names sorted alphabetically.
func AllToolNames() []string {
	names := make([]string, 0, len(ToolRegistry))
	for name := range ToolRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// StandardFallback is always used as a skill mount target, created if needed.
var StandardFallback = ToolDir{Dir: ".agents", SkillsSubdir: "skills"}

// ResolveToolDirs determines which tool directories to use by combining:
// 1. Configured tools (from config.yml) — the baseline
// 2. Auto-detected tool directories in the project — merged in
// 3. .agents/ fallback — always included
func ResolveToolDirs(projectRoot string, configuredTools []string) []ToolDir {
	found := make(map[string]bool)
	var dirs []ToolDir

	// Start with configured tools
	for _, name := range configuredTools {
		if tool, ok := ToolRegistry[name]; ok {
			if !found[tool.Dir] {
				found[tool.Dir] = true
				dirs = append(dirs, tool)
			}
		}
	}

	// Add auto-detected tool dirs (even if not in config)
	for _, tool := range ToolRegistry {
		if found[tool.Dir] {
			continue
		}
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

// isAgentEntry returns true if the source entry is a subagent definition
// under the agents/ directory (e.g., agents/code-reviewer.md, agents/planner.md).
// These are projected into tool-native agent directories (.claude/agents/, etc.).
// Root-level AGENTS.md is NOT an agent entry — it mounts to project root.
func isAgentEntry(entry SourceEntry) bool {
	return strings.HasPrefix(entry.RelativePath, "agents/") && strings.HasSuffix(entry.RelativePath, ".md")
}

// isRuleEntry returns true if the source entry is a rule/instruction file
// under the rules/ directory (e.g., rules/go-conventions.md).
// These are projected into tool-native rule directories (.claude/rules/, etc.).
func isRuleEntry(entry SourceEntry) bool {
	return strings.HasPrefix(entry.RelativePath, "rules/") && strings.HasSuffix(entry.RelativePath, ".md")
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
		if isSkillEntry(entry) {
			if len(toolDirs) == 0 {
				expanded = append(expanded, entry)
				continue
			}
			name := skillName(entry)
			for _, tool := range toolDirs {
				if tool.SkillsSubdir == "" {
					continue
				}
				toolRelPath := filepath.ToSlash(
					filepath.Join(tool.Dir, tool.SkillsSubdir, name, "SKILL.md"),
				)
				expanded = append(expanded, SourceEntry{
					SourceName:    entry.SourceName,
					RelativePath:  toolRelPath,
					QualifiedPath: entry.QualifiedPath + "→" + toolRelPath,
					FullPath:      entry.FullPath,
					Priority:      entry.Priority,
				})
			}
			continue
		}

		if isAgentEntry(entry) {
			// Mount subagent definitions into tool agent directories
			// (e.g., agents/code-reviewer.md → .claude/agents/code-reviewer.md)
			expanded = appendProjected(expanded, entry, toolDirs, func(t ToolDir) string { return t.AgentsSubdir })
			continue
		}

		if isRuleEntry(entry) {
			// Mount rule files into tool rule directories
			// (e.g., rules/go-conventions.md → .claude/rules/go-conventions.md)
			expanded = appendProjected(expanded, entry, toolDirs, func(t ToolDir) string { return t.RulesSubdir })
			continue
		}

		// Everything else (AGENTS.md, CLAUDE.md, etc.): keep at original path
		expanded = append(expanded, entry)
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

// appendProjected projects a registry entry into tool-native directories using
// the provided subdir selector. If no tool has the relevant subdir, the entry
// is kept at its original path.
func appendProjected(expanded []SourceEntry, entry SourceEntry, toolDirs []ToolDir, subdirFn func(ToolDir) string) []SourceEntry {
	baseName := filepath.Base(entry.RelativePath)
	mounted := false
	for _, tool := range toolDirs {
		subdir := subdirFn(tool)
		if subdir == "" {
			continue
		}
		toolRelPath := filepath.ToSlash(
			filepath.Join(tool.Dir, subdir, baseName),
		)
		expanded = append(expanded, SourceEntry{
			SourceName:    entry.SourceName,
			RelativePath:  toolRelPath,
			QualifiedPath: entry.QualifiedPath + "→" + toolRelPath,
			FullPath:      entry.FullPath,
			Priority:      entry.Priority,
		})
		mounted = true
	}
	if !mounted {
		expanded = append(expanded, entry)
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

// AdoptableFile describes a file in a project that can be adopted into a registry.
type AdoptableFile struct {
	ProjectPath  string // relative path in project (e.g., ".claude/skills/foo/SKILL.md")
	RegistryPath string // where it should go in the registry (e.g., "skills/foo/SKILL.md")
}

// ScanAdoptableFiles scans a project directory for AI config files that can be
// adopted into a csaw registry. This is the reverse of ExpandToolTargets —
// it maps tool-native paths back to registry-standard paths.
func ScanAdoptableFiles(projectRoot string) []AdoptableFile {
	var files []AdoptableFile
	seen := make(map[string]bool) // registry path → already found

	// Root-level instruction files
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		path := filepath.Join(projectRoot, name)
		if _, err := os.Stat(path); err == nil {
			files = append(files, AdoptableFile{ProjectPath: name, RegistryPath: name})
			seen[name] = true
		}
	}

	// Skills from tool directories (reverse: .claude/skills/foo/SKILL.md → skills/foo/SKILL.md)
	for _, tool := range ToolRegistry {
		if tool.SkillsSubdir == "" {
			continue
		}
		skillsDir := filepath.Join(projectRoot, tool.Dir, tool.SkillsSubdir)
		entries, err := os.ReadDir(skillsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}
			registryPath := "skills/" + entry.Name() + "/SKILL.md"
			if seen[registryPath] {
				continue
			}
			seen[registryPath] = true
			files = append(files, AdoptableFile{
				ProjectPath:  filepath.ToSlash(filepath.Join(tool.Dir, tool.SkillsSubdir, entry.Name(), "SKILL.md")),
				RegistryPath: registryPath,
			})
		}
	}

	// Rules from tool rule directories (reverse: .claude/rules/go.md → rules/go.md)
	for _, tool := range ToolRegistry {
		if tool.RulesSubdir == "" {
			continue
		}
		rulesDir := filepath.Join(projectRoot, tool.Dir, tool.RulesSubdir)
		entries, err := os.ReadDir(rulesDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			registryPath := "rules/" + entry.Name()
			if seen[registryPath] {
				continue
			}
			seen[registryPath] = true
			files = append(files, AdoptableFile{
				ProjectPath:  filepath.ToSlash(filepath.Join(tool.Dir, tool.RulesSubdir, entry.Name())),
				RegistryPath: registryPath,
			})
		}
	}

	// Subagent definitions from tool agent directories (reverse: .claude/agents/reviewer.md → agents/reviewer.md)
	for _, tool := range ToolRegistry {
		if tool.AgentsSubdir == "" {
			continue
		}
		agentsDir := filepath.Join(projectRoot, tool.Dir, tool.AgentsSubdir)
		entries, err := os.ReadDir(agentsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			registryPath := "agents/" + entry.Name()
			if seen[registryPath] {
				continue
			}
			seen[registryPath] = true
			files = append(files, AdoptableFile{
				ProjectPath:  filepath.ToSlash(filepath.Join(tool.Dir, tool.AgentsSubdir, entry.Name())),
				RegistryPath: registryPath,
			})
		}
	}

	// MCP configs (reverse: .mcp.json → mcp/claude-code.json)
	for _, target := range KnownMCPTargets {
		path := filepath.Join(projectRoot, filepath.FromSlash(target.ProjectPath))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		registryPath := "mcp/" + target.RegistryFile
		if seen[registryPath] {
			continue
		}
		seen[registryPath] = true
		files = append(files, AdoptableFile{
			ProjectPath:  target.ProjectPath,
			RegistryPath: registryPath,
		})
	}

	return files
}
