package mount

//go:generate go run github.com/NicholasCullenCooper/csaw/cmd/tools-gen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Kind classifies a registry entry by what kind of AI workspace artifact it is.
// Each kind has its own conventions for where files live in the registry and
// where they project into tool directories.
type Kind string

const (
	KindAgent       Kind = "agent"
	KindSkill       Kind = "skill"
	KindRule        Kind = "rule"
	KindMCP         Kind = "mcp"
	KindInstruction Kind = "instruction"
	KindHook        Kind = "hook"
	KindIgnore      Kind = "ignore"
	KindOther       Kind = "other"
)

// AllKinds returns the set of user-selectable kinds, in display order.
var AllKinds = []Kind{KindAgent, KindSkill, KindRule, KindMCP, KindInstruction, KindHook, KindIgnore}

// KindOf classifies a registry-side source entry by inspecting its path.
func KindOf(entry SourceEntry) Kind {
	switch {
	case isAgentEntry(entry):
		return KindAgent
	case isSkillEntry(entry):
		return KindSkill
	case isRuleEntry(entry):
		return KindRule
	case isMCPEntry(entry):
		return KindMCP
	case isHookEntry(entry):
		return KindHook
	case isIgnoreEntry(entry):
		return KindIgnore
	case isInstructionEntry(entry):
		return KindInstruction
	default:
		return KindOther
	}
}

// isInstructionEntry returns true for top-level instruction files like
// AGENTS.md, CLAUDE.md that mount to the project root and are always loaded.
// Also recognizes tool-specific project-root instruction files: GEMINI.md
// (Gemini CLI / Antigravity), QWEN.md (Qwen Code), and .goosehints (Goose).
func isInstructionEntry(entry SourceEntry) bool {
	rel := entry.RelativePath
	if strings.Contains(rel, "/") {
		return false
	}
	switch rel {
	case "AGENTS.md", "CLAUDE.md", "AGENT.md", "GEMINI.md", "QWEN.md", ".goosehints":
		return true
	}
	return false
}

// isHookEntry returns true for entries under the registry's hooks/ directory.
// These project into per-tool hook directories (e.g., .claude/hooks/, .kiro/hooks/).
func isHookEntry(entry SourceEntry) bool {
	return strings.HasPrefix(entry.RelativePath, "hooks/")
}

// isIgnoreEntry returns true for entries under the registry's ignore/ directory.
// Each file projects to a single tool-specific ignore path at project root
// (e.g., ignore/cursor → .cursorignore, ignore/cody → .cody/ignore).
func isIgnoreEntry(entry SourceEntry) bool {
	rel := entry.RelativePath
	dir := filepath.Dir(rel)
	return dir == "ignore"
}

// ParseKind maps a user-supplied name (singular or plural) to a Kind value.
func ParseKind(s string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "agent", "agents":
		return KindAgent, nil
	case "skill", "skills":
		return KindSkill, nil
	case "rule", "rules":
		return KindRule, nil
	case "hook", "hooks":
		return KindHook, nil
	case "ignore", "ignores":
		return KindIgnore, nil
	case "mcp", "mcps":
		return KindMCP, nil
	case "instruction", "instructions":
		return KindInstruction, nil
	default:
		return "", fmt.Errorf("unknown kind %q (valid: agents, skills, rules, mcp, instructions)", s)
	}
}

// KindLabel returns the user-facing plural label for a Kind.
func KindLabel(k Kind) string {
	switch k {
	case KindAgent:
		return "agents"
	case KindSkill:
		return "skills"
	case KindRule:
		return "rules"
	case KindMCP:
		return "mcp"
	case KindInstruction:
		return "instructions"
	case KindHook:
		return "hooks"
	case KindIgnore:
		return "ignore"
	default:
		return "other"
	}
}

// KindOfProjectPath classifies a project-relative mounted path by destination.
// This is the post-projection classifier used by inspect to group mounted
// files by what they are.
func KindOfProjectPath(rel string) Kind {
	rel = filepath.ToSlash(rel)

	if !strings.Contains(rel, "/") {
		switch rel {
		case "AGENTS.md", "CLAUDE.md", "AGENT.md", "GEMINI.md", "QWEN.md", ".goosehints":
			return KindInstruction
		}
	}

	for _, target := range KnownIgnoreTargets {
		if rel == target.ProjectPath {
			return KindIgnore
		}
	}

	for _, target := range KnownMCPTargets {
		if rel == target.ProjectPath {
			return KindMCP
		}
	}
	base := filepath.Base(rel)
	if base == "mcp.json" || base == ".mcp.json" {
		return KindMCP
	}

	for _, tool := range ToolRegistry {
		prefix := tool.Dir + "/"
		if !strings.HasPrefix(rel, prefix) {
			continue
		}
		rest := strings.TrimPrefix(rel, prefix)
		if tool.AgentsSubdir != "" && strings.HasPrefix(rest, tool.AgentsSubdir+"/") {
			return KindAgent
		}
		if tool.SkillsSubdir != "" && strings.HasPrefix(rest, tool.SkillsSubdir+"/") {
			return KindSkill
		}
		if tool.RulesSubdir != "" && strings.HasPrefix(rest, tool.RulesSubdir+"/") {
			return KindRule
		}
		if tool.HooksSubdir != "" && strings.HasPrefix(rest, tool.HooksSubdir+"/") {
			return KindHook
		}
	}

	if strings.HasPrefix(rel, StandardFallback.Dir+"/") {
		rest := strings.TrimPrefix(rel, StandardFallback.Dir+"/")
		if StandardFallback.SkillsSubdir != "" && strings.HasPrefix(rest, StandardFallback.SkillsSubdir+"/") {
			return KindSkill
		}
	}

	return KindOther
}

// ToolDir describes a tool's directory conventions for skills, rules, agents, and hooks.
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
	// HooksSubdir is the path under Dir where lifecycle hook scripts are stored.
	// Empty means this tool doesn't have a file-based hooks directory.
	HooksSubdir string
}

// ToolRegistry maps short tool names to their directory conventions.
//
// Per-tool support matrix is derived from docs/reference/tool-projection.json.
// Tools NOT included and why:
//   - codex has no `.codex/agents/` (uses TOML inline tables in config.toml)
//   - cursor has no `.cursor/agents/` (Background Agents are runtime, not files)
//   - goose has `.goose/recipes/` (YAML, not csaw's SKILL.md folder pattern) and
//     `.goosehints` (project-root single file); needs new kind support — pending.
//
// DEPRECATIONS:
//   - gemini: Google sunsets 2026-06-18; users should migrate to antigravity which
//     reuses `.agents/` (also csaw's StandardFallback path).
var ToolRegistry = map[string]ToolDir{
	"claude":      {Dir: ".claude", SkillsSubdir: "skills", RulesSubdir: "rules", AgentsSubdir: "agents", HooksSubdir: "hooks"},
	"opencode":    {Dir: ".opencode", SkillsSubdir: "skills", AgentsSubdir: "agents"},
	"codex":       {Dir: ".codex", SkillsSubdir: "skills"}, // hooks live in config.toml [hooks] — not file-projectable
	"cursor":      {Dir: ".cursor", RulesSubdir: "rules"},
	"windsurf":    {Dir: ".windsurf", RulesSubdir: "rules"},
	"gemini":      {Dir: ".gemini", AgentsSubdir: "agents"}, // DEPRECATED: sunset 2026-06-18.
	"antigravity": {Dir: ".agents", SkillsSubdir: "skills"}, // Replaces gemini. Same path as StandardFallback.
	"amazon-q":    {Dir: ".amazonq", RulesSubdir: "rules"},
	"kiro":        {Dir: ".kiro", RulesSubdir: "steering", AgentsSubdir: "agents", HooksSubdir: "hooks"},
	"codebuddy":   {Dir: ".codebuddy", RulesSubdir: "rules", AgentsSubdir: "agents"},
	"openhands":   {Dir: ".openhands", AgentsSubdir: "microagents"},
	"goose":       {Dir: ".goose"}, // .goosehints handled at instruction layer; recipes are user-scope.
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
	// Second pass: project ignore files to tool-specific paths.
	entries = expandIgnoreTargets(entries)

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
					Protected:     entry.Protected,
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

		if isHookEntry(entry) {
			// Mount hook scripts into tool hook directories
			// (e.g., hooks/pre-commit.sh → .claude/hooks/pre-commit.sh)
			expanded = appendProjected(expanded, entry, toolDirs, func(t ToolDir) string { return t.HooksSubdir })
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

// IgnoreTarget maps a registry filename under ignore/ to a project-relative
// path where the corresponding tool expects its ignore file.
type IgnoreTarget struct {
	// RegistryFile is the filename in the registry's ignore/ directory
	// (e.g., "cursor" for ignore/cursor).
	RegistryFile string
	// ProjectPath is the relative path in the project (e.g., ".cursorignore").
	ProjectPath string
}

// KnownIgnoreTargets lists the supported per-tool ignore file projections.
// Each entry maps a file in the registry's ignore/ directory to the path the
// corresponding tool reads. Uses gitignore-style syntax in all cases.
var KnownIgnoreTargets = []IgnoreTarget{
	{RegistryFile: "cursor", ProjectPath: ".cursorignore"},
	{RegistryFile: "cody", ProjectPath: ".cody/ignore"},
	{RegistryFile: "aider", ProjectPath: ".aiderignore"},
	{RegistryFile: "lingma", ProjectPath: ".tongyiignore"},
}

// ignoreProjectPath returns the project-relative target path for an ignore
// entry, or empty string if the filename is not a known target.
func ignoreProjectPath(entry SourceEntry) string {
	base := filepath.Base(entry.RelativePath)
	for _, target := range KnownIgnoreTargets {
		if base == target.RegistryFile {
			return target.ProjectPath
		}
	}
	return ""
}

// expandIgnoreTargets redirects ignore entries from their registry paths
// (ignore/cursor) to tool-specific project paths (.cursorignore). Unknown
// ignore files are kept at their original path.
func expandIgnoreTargets(entries []SourceEntry) []SourceEntry {
	var expanded []SourceEntry
	for _, entry := range entries {
		if !isIgnoreEntry(entry) {
			expanded = append(expanded, entry)
			continue
		}
		projectPath := ignoreProjectPath(entry)
		if projectPath == "" {
			expanded = append(expanded, entry)
			continue
		}
		expanded = append(expanded, SourceEntry{
			SourceName:    entry.SourceName,
			RelativePath:  projectPath,
			QualifiedPath: entry.QualifiedPath + "→" + projectPath,
			FullPath:      entry.FullPath,
			Priority:      entry.Priority,
			Protected:     entry.Protected,
		})
	}
	return expanded
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
			Priority:      entry.Priority,
			Protected:     entry.Protected,
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
			Protected:     entry.Protected,
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
