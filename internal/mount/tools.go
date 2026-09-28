package mount

//go:generate go run github.com/NicholasCullenCooper/csaw/cmd/tools-gen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NicholasCullenCooper/csaw/internal/hooks"
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

// instructionFiles lists the project-root instruction files csaw recognizes.
// AGENTS.md is the canonical cross-tool file (Codex reads it directly);
// CLAUDE.md is what Claude Code reads. The rest are tool-specific files some
// registries carry verbatim.
var instructionFiles = []string{"AGENTS.md", "CLAUDE.md", "AGENT.md", "GEMINI.md", "QWEN.md", ".goosehints"}

func isInstructionFile(name string) bool {
	for _, candidate := range instructionFiles {
		if name == candidate {
			return true
		}
	}
	return false
}

// isInstructionEntry returns true for top-level instruction files like
// AGENTS.md, CLAUDE.md that mount to the project root and are always loaded.
func isInstructionEntry(entry SourceEntry) bool {
	rel := entry.RelativePath
	if strings.Contains(rel, "/") {
		return false
	}
	return isInstructionFile(rel)
}

// isHookEntry returns true for entries under the registry's hooks/ directory.
// These project into per-tool hook script directories (e.g., .claude/hooks/).
// Scripts are placed only; declaring them in the tool's settings is a separate
// step until the hooks kind is reworked (see docs/exec-plans/active/relaunch-2026-09.md).
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
		return "", fmt.Errorf("unknown kind %q (valid: agents, skills, rules, mcp, instructions, hooks, ignore)", s)
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

// ToolSpec describes what one AI coding tool reads from a project and where
// csaw writes to serve it.
//
// Reads lists, per kind, every project-relative directory the tool scans:
// its native path first, then compatibility paths it reads from other tools.
// Writes names the single directory csaw uses when this tool must be served
// and no directory chosen for another tool already covers it. PlanTargets
// combines the two into the smallest set of directories that reaches every
// configured tool.
//
// csaw supports Claude Code and Codex. Other tools are catalogued with their
// research in docs/reference/tool-projection.json but are not projected.
type ToolSpec struct {
	Name        string
	DisplayName string
	// DetectPaths are project-relative paths whose presence implies the tool
	// is in use, for auto-detection when no tools are configured.
	DetectPaths []string
	// InstructionFile is the project-root instruction file the tool reads.
	InstructionFile string
	// BridgeImport, when set, lets csaw generate InstructionFile from a
	// registry's AGENTS.md using the tool's import syntax (e.g. "@AGENTS.md").
	BridgeImport string
	// HooksMerge names the hooks.Targets entry a registry hooks/hooks.json is
	// merged into for this tool. Empty means the tool has no hooks file.
	HooksMerge string
	Reads      map[Kind][]string
	Writes     map[Kind]string
}

// ReadsKind reports whether the tool reads any project directory for kind.
func (t ToolSpec) ReadsKind(kind Kind) bool {
	return len(t.Reads[kind]) > 0
}

func (t ToolSpec) readsDir(kind Kind, dir string) bool {
	for _, candidate := range t.Reads[kind] {
		if candidate == dir {
			return true
		}
	}
	return false
}

// neutralSkills is the tool-neutral skills directory Codex reads (as do most
// other agents). Claude Code reads only .claude/skills.
const neutralSkills = ".agents/skills"

// ToolRegistry maps short tool names to their read and write specs.
//
// Paths were verified against vendor docs on 2026-09-14/15; see
// docs/reference/tool-projection.json for the research behind each entry and
// docs/reference/tool-projection.md for the rendered projection table.
var ToolRegistry = map[string]ToolSpec{
	"claude": {
		Name:            "claude",
		DisplayName:     "Claude Code",
		DetectPaths:     []string{".claude"},
		InstructionFile: "CLAUDE.md",
		BridgeImport:    "@AGENTS.md",
		HooksMerge:      "claude",
		Reads: map[Kind][]string{
			KindSkill: {".claude/skills"},
			KindAgent: {".claude/agents"},
			KindRule:  {".claude/rules"},
			KindHook:  {".claude/hooks"},
		},
		Writes: map[Kind]string{
			KindSkill: ".claude/skills",
			KindAgent: ".claude/agents",
			KindRule:  ".claude/rules",
			KindHook:  ".claude/hooks",
		},
	},
	"codex": {
		Name:            "codex",
		DisplayName:     "Codex (OpenAI / ChatGPT)",
		DetectPaths:     []string{".codex"},
		InstructionFile: "AGENTS.md",
		HooksMerge:      "codex",
		Reads: map[Kind][]string{
			KindSkill: {neutralSkills},
			KindHook:  {".codex/hooks"},
		},
		Writes: map[Kind]string{
			KindSkill: neutralSkills,
			KindHook:  ".codex/hooks",
		},
	},
}

// hooksConfigPath is the registry file whose contents csaw merges into each
// tool's hooks file rather than copying.
const hooksConfigPath = "hooks/hooks.json"

// StandardFallback is used when no tool is configured or detected: skills
// land in the tool-neutral .agents/ directory that Codex (and most other
// agents) read.
var StandardFallback = ToolSpec{
	Name:            "agents",
	DisplayName:     "Neutral .agents/ directory",
	InstructionFile: "AGENTS.md",
	Reads: map[Kind][]string{
		KindSkill: {neutralSkills},
	},
	Writes: map[Kind]string{
		KindSkill: neutralSkills,
	},
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

// ToolDisplayName returns the human-readable name for a tool key, or the key
// itself when unknown.
func ToolDisplayName(key string) string {
	if spec, ok := ToolRegistry[key]; ok {
		return spec.DisplayName
	}
	return key
}

// ResolveTools determines which tools to project for by combining:
// 1. Configured tools (from config.yml or --tools)
// 2. Auto-detected tools whose DetectPaths exist in the project
// 3. StandardFallback when nothing else applies
//
// The result is sorted by name so projection output is deterministic.
func ResolveTools(projectRoot string, configuredTools []string) []ToolSpec {
	seen := make(map[string]bool)
	var tools []ToolSpec

	for _, name := range configuredTools {
		spec, ok := ToolRegistry[name]
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		tools = append(tools, spec)
	}

	for _, name := range AllToolNames() {
		if seen[name] {
			continue
		}
		spec := ToolRegistry[name]
		for _, detect := range spec.DetectPaths {
			if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(detect))); err == nil {
				seen[name] = true
				tools = append(tools, spec)
				break
			}
		}
	}

	if len(tools) == 0 {
		return []ToolSpec{StandardFallback}
	}

	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

// PlanTargets returns the smallest set of directories for kind such that
// every tool that reads kind is served at least once. It is a greedy set
// cover over each tool's Reads: the candidate directory reaching the most
// still-uncovered tools wins each round, ties broken by directory name so
// results are deterministic. Tools that read nothing for kind are skipped.
func PlanTargets(kind Kind, tools []ToolSpec) []string {
	var needing []ToolSpec
	candidates := make(map[string]bool)
	for _, tool := range tools {
		if !tool.ReadsKind(kind) {
			continue
		}
		needing = append(needing, tool)
		if dir := tool.Writes[kind]; dir != "" {
			candidates[dir] = true
		}
	}

	covered := make(map[string]bool)
	var chosen []string
	for len(covered) < len(needing) {
		bestDir := ""
		bestCount := 0
		for _, dir := range sortedKeys(candidates) {
			count := 0
			for _, tool := range needing {
				if !covered[tool.Name] && tool.readsDir(kind, dir) {
					count++
				}
			}
			if count > bestCount {
				bestCount = count
				bestDir = dir
			}
		}
		if bestCount == 0 {
			break
		}
		chosen = append(chosen, bestDir)
		for _, tool := range needing {
			if tool.readsDir(kind, bestDir) {
				covered[tool.Name] = true
			}
		}
		delete(candidates, bestDir)
	}

	sort.Strings(chosen)
	return chosen
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// BridgeContent renders the generated instruction file for a tool that does
// not read AGENTS.md. Claude Code strips the HTML comment before the file
// enters context.
func BridgeContent(tool ToolSpec) string {
	return fmt.Sprintf("<!-- csaw-managed: generated because %s reads %s, not AGENTS.md. Edit AGENTS.md in its source registry; csaw unmount removes this file. -->\n%s\n",
		tool.DisplayName, tool.InstructionFile, tool.BridgeImport)
}

// skillParts splits a registry path under skills/ into the skill name and the
// path of the file inside the skill directory. Any file under
// skills/<name>/ (or skills/experimental/<name>/) belongs to skill <name>,
// so supporting files such as references/ and scripts/ travel with SKILL.md.
func skillParts(rel string) (name string, rest string, ok bool) {
	segments := strings.Split(rel, "/")
	index := -1
	for i, segment := range segments {
		if segment == "skills" {
			index = i
			break
		}
	}
	if index < 0 {
		return "", "", false
	}
	nameIndex := index + 1
	if nameIndex < len(segments) && segments[nameIndex] == "experimental" {
		nameIndex++
	}
	if nameIndex >= len(segments)-1 {
		return "", "", false
	}
	return segments[nameIndex], strings.Join(segments[nameIndex+1:], "/"), true
}

// isSkillEntry returns true if the source entry belongs to a skill directory
// (any file under skills/<name>/).
func isSkillEntry(entry SourceEntry) bool {
	_, _, ok := skillParts(entry.RelativePath)
	return ok
}

// isAgentEntry returns true if the source entry is a subagent definition
// under the agents/ directory (e.g., agents/code-reviewer.md, agents/planner.md).
// Root-level AGENTS.md is NOT an agent entry — it mounts to project root.
func isAgentEntry(entry SourceEntry) bool {
	return strings.HasPrefix(entry.RelativePath, "agents/") && strings.HasSuffix(entry.RelativePath, ".md")
}

// isRuleEntry returns true if the source entry is a rule/instruction file
// under the rules/ directory (e.g., rules/go-conventions.md).
func isRuleEntry(entry SourceEntry) bool {
	return strings.HasPrefix(entry.RelativePath, "rules/") && strings.HasSuffix(entry.RelativePath, ".md")
}

// ExpandToolTargets redirects registry entries into the project paths the
// configured tools read.
//
//   - MCP and ignore files map to fixed per-tool paths.
//   - Skill directories, agents, rules, and hook scripts are projected into the
//     directories chosen by PlanTargets, so each configured tool sees every
//     file exactly once.
//   - Instruction files stay at the project root. When a tool reads a different
//     root file than AGENTS.md (Claude Code reads CLAUDE.md) and the registry
//     does not provide it, csaw adds a generated bridge file that imports
//     AGENTS.md.
//
// With no tools, entries are returned after MCP and ignore projection only.
func ExpandToolTargets(entries []SourceEntry, tools []ToolSpec) []SourceEntry {
	entries = expandMCPTargets(entries)
	entries = expandIgnoreTargets(entries)
	if len(tools) == 0 {
		return entries
	}

	targets := map[Kind][]string{
		KindSkill: PlanTargets(KindSkill, tools),
		KindAgent: PlanTargets(KindAgent, tools),
		KindRule:  PlanTargets(KindRule, tools),
		KindHook:  PlanTargets(KindHook, tools),
	}

	var expanded []SourceEntry
	present := make(map[string]SourceEntry)
	for _, entry := range entries {
		if name, rest, ok := skillParts(entry.RelativePath); ok {
			if len(targets[KindSkill]) == 0 {
				expanded = append(expanded, entry)
				continue
			}
			for _, dir := range targets[KindSkill] {
				expanded = append(expanded, projectEntry(entry, path.Join(dir, name, rest)))
			}
			continue
		}

		switch {
		case entry.RelativePath == hooksConfigPath:
			expanded = append(expanded, planHookMerges(entry, tools)...)
		case isAgentEntry(entry):
			expanded = appendProjected(expanded, entry, targets[KindAgent])
		case isRuleEntry(entry):
			expanded = appendProjected(expanded, entry, targets[KindRule])
		case isHookEntry(entry):
			// Scripts keep their path under hooks/ (hooks/scripts/x.sh →
			// .claude/hooks/scripts/x.sh) so rewritten commands find them.
			rest := strings.TrimPrefix(entry.RelativePath, "hooks/")
			if len(targets[KindHook]) == 0 {
				expanded = append(expanded, entry)
				continue
			}
			for _, dir := range targets[KindHook] {
				expanded = append(expanded, projectEntry(entry, path.Join(dir, rest)))
			}
		default:
			if isInstructionEntry(entry) {
				present[entry.RelativePath] = entry
			}
			expanded = append(expanded, entry)
		}
	}

	if canonical, ok := present["AGENTS.md"]; ok {
		bridged := make(map[string]bool)
		for _, tool := range sortedTools(tools) {
			file := tool.InstructionFile
			if tool.BridgeImport == "" || file == "" || file == "AGENTS.md" {
				continue
			}
			if _, exists := present[file]; exists || bridged[file] {
				continue
			}
			bridged[file] = true
			expanded = append(expanded, SourceEntry{
				SourceName:    canonical.SourceName,
				RelativePath:  file,
				QualifiedPath: canonical.QualifiedPath + "→" + file,
				FullPath:      canonical.FullPath,
				Priority:      canonical.Priority,
				Protected:     canonical.Protected,
				Generated:     true,
				Content:       BridgeContent(tool),
			})
		}
	}

	return expanded
}

// planHookMerges turns a registry hooks/hooks.json into one merge entry per
// configured tool that has a hooks file. The registry file is read here so
// the plan can drop events and handler types a tool does not support; those
// decisions are returned as Notes on the entries. A registry file that does
// not parse becomes a single note and no merge.
func planHookMerges(entry SourceEntry, tools []ToolSpec) []SourceEntry {
	cfg, err := hooks.Load(entry.FullPath)
	if err != nil {
		return []SourceEntry{{
			SourceName:    entry.SourceName,
			RelativePath:  entry.RelativePath,
			QualifiedPath: entry.QualifiedPath,
			FullPath:      entry.FullPath,
			Priority:      entry.Priority,
			Protected:     entry.Protected,
			MergeHooks:    true,
			Notes:         []string{fmt.Sprintf("%s: %v; hooks not projected", entry.QualifiedPath, err)},
		}}
	}
	var out []SourceEntry
	for _, tool := range sortedTools(tools) {
		target, ok := hooks.Targets[tool.HooksMerge]
		if !ok {
			continue
		}
		owned, notes := hooks.Plan(cfg, target)
		if hooks.Count(owned) == 0 {
			notes = append(notes, fmt.Sprintf("%s: nothing %s can run; skipped", entry.QualifiedPath, tool.DisplayName))
			out = append(out, SourceEntry{RelativePath: target.Path, MergeHooks: true, Notes: notes, Content: ""})
			continue
		}
		merged := projectEntry(entry, target.Path)
		merged.MergeHooks = true
		merged.Content = hooks.Encode(owned)
		merged.Notes = notes
		out = append(out, merged)
	}
	return out
}

func sortedTools(tools []ToolSpec) []ToolSpec {
	sorted := append([]ToolSpec(nil), tools...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	return sorted
}

func projectEntry(entry SourceEntry, rel string) SourceEntry {
	return SourceEntry{
		SourceName:    entry.SourceName,
		RelativePath:  rel,
		QualifiedPath: entry.QualifiedPath + "→" + rel,
		FullPath:      entry.FullPath,
		Priority:      entry.Priority,
		Protected:     entry.Protected,
	}
}

// appendProjected projects a single-file registry entry into each target
// directory. With no targets the entry is kept at its original path.
func appendProjected(expanded []SourceEntry, entry SourceEntry, dirs []string) []SourceEntry {
	if len(dirs) == 0 {
		return append(expanded, entry)
	}
	base := filepath.Base(entry.RelativePath)
	for _, dir := range dirs {
		expanded = append(expanded, projectEntry(entry, path.Join(dir, base)))
	}
	return expanded
}

// KindOfProjectPath classifies a project-relative mounted path by destination.
// This is the post-projection classifier used by inspect to group mounted
// files by what they are.
func KindOfProjectPath(rel string) Kind {
	rel = filepath.ToSlash(rel)

	if !strings.Contains(rel, "/") && isInstructionFile(rel) {
		return KindInstruction
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

	for _, target := range hooks.Targets {
		if rel == target.Path {
			return KindHook
		}
	}

	for _, dk := range projectDirKinds() {
		if strings.HasPrefix(rel, dk.dir+"/") {
			return dk.kind
		}
	}

	return KindOther
}

type dirKind struct {
	dir  string
	kind Kind
}

// projectDirKinds returns every directory any known tool reads or csaw
// writes, tagged with its kind and sorted longest-first so nested directories
// win prefix matching.
func projectDirKinds() []dirKind {
	seen := make(map[string]Kind)
	add := func(kind Kind, dir string) {
		if dir != "" {
			seen[dir] = kind
		}
	}
	for _, spec := range append(allToolSpecs(), StandardFallback) {
		for kind, dirs := range spec.Reads {
			for _, dir := range dirs {
				add(kind, dir)
			}
		}
		for kind, dir := range spec.Writes {
			add(kind, dir)
		}
	}
	out := make([]dirKind, 0, len(seen))
	for dir, kind := range seen {
		out = append(out, dirKind{dir: dir, kind: kind})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].dir) != len(out[j].dir) {
			return len(out[i].dir) > len(out[j].dir)
		}
		return out[i].dir < out[j].dir
	})
	return out
}

func allToolSpecs() []ToolSpec {
	specs := make([]ToolSpec, 0, len(ToolRegistry))
	for _, name := range AllToolNames() {
		specs = append(specs, ToolRegistry[name])
	}
	return specs
}

// KnownTopLevelDirs returns the first path segment of every directory csaw
// may write or a tool may read (".claude", ".agents", ...), sorted.
func KnownTopLevelDirs() []string {
	seen := make(map[string]bool)
	for _, dk := range projectDirKinds() {
		first, _, _ := strings.Cut(dk.dir, "/")
		seen[first] = true
	}
	return sortedKeys(seen)
}

// readDirsForKind returns every directory any known tool reads for kind,
// deduplicated and sorted.
func readDirsForKind(kind Kind) []string {
	seen := make(map[string]bool)
	for _, spec := range append(allToolSpecs(), StandardFallback) {
		for _, dir := range spec.Reads[kind] {
			seen[dir] = true
		}
	}
	return sortedKeys(seen)
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
// Claude Code reads .mcp.json. Codex keeps MCP inside .codex/config.toml,
// which `csaw mcp sync codex` merges rather than links.
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
// corresponding tool reads. Uses gitignore-style syntax in all cases. Neither
// Claude Code nor Codex has a project ignore file; these serve other tools a
// registry may still carry files for.
var KnownIgnoreTargets = []IgnoreTarget{
	{RegistryFile: "cursor", ProjectPath: ".cursorignore"},
	{RegistryFile: "cody", ProjectPath: ".cody/ignore"},
	{RegistryFile: "aider", ProjectPath: ".aiderignore"},
	{RegistryFile: "lingma", ProjectPath: ".tongyiignore"},
	{RegistryFile: "gemini", ProjectPath: ".geminiignore"},
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
		expanded = append(expanded, projectEntry(entry, projectPath))
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
		expanded = append(expanded, projectEntry(entry, projectPath))
	}
	return expanded
}

// AdoptableFile describes a file in a project that can be adopted into a registry.
type AdoptableFile struct {
	ProjectPath  string // relative path in project (e.g., ".claude/skills/foo/SKILL.md")
	RegistryPath string // where it should go in the registry (e.g., "skills/foo/SKILL.md")
}

// ScanAdoptableFiles scans a project directory for AI config files that can be
// adopted into a csaw registry. This is the reverse of ExpandToolTargets —
// it maps tool-native paths back to registry-standard paths. Every directory
// a supported tool reads is scanned.
func ScanAdoptableFiles(projectRoot string) []AdoptableFile {
	var files []AdoptableFile
	seen := make(map[string]bool) // registry path → already found

	add := func(projectPath, registryPath string) {
		if seen[registryPath] {
			return
		}
		seen[registryPath] = true
		files = append(files, AdoptableFile{ProjectPath: filepath.ToSlash(projectPath), RegistryPath: registryPath})
	}

	// Root-level instruction files
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(projectRoot, name)); err == nil {
			add(name, name)
		}
	}

	// Whole skill directories (reverse: .claude/skills/foo/** → skills/foo/**)
	for _, dir := range readDirsForKind(KindSkill) {
		skillsDir := filepath.Join(projectRoot, filepath.FromSlash(dir))
		entries, err := os.ReadDir(skillsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillRoot := filepath.Join(skillsDir, entry.Name())
			if _, err := os.Stat(filepath.Join(skillRoot, "SKILL.md")); err != nil {
				continue
			}
			filepath.WalkDir(skillRoot, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				rest, err := filepath.Rel(skillRoot, p)
				if err != nil {
					return nil
				}
				rest = filepath.ToSlash(rest)
				add(path.Join(dir, entry.Name(), rest), path.Join("skills", entry.Name(), rest))
				return nil
			})
		}
	}

	// Rules (reverse: .claude/rules/go.md → rules/go.md)
	for _, dir := range readDirsForKind(KindRule) {
		for _, name := range markdownFilesIn(filepath.Join(projectRoot, filepath.FromSlash(dir))) {
			add(path.Join(dir, name), path.Join("rules", name))
		}
	}

	// Subagent definitions (reverse: .claude/agents/reviewer.md → agents/reviewer.md)
	for _, dir := range readDirsForKind(KindAgent) {
		for _, name := range markdownFilesIn(filepath.Join(projectRoot, filepath.FromSlash(dir))) {
			add(path.Join(dir, name), path.Join("agents", name))
		}
	}

	// MCP configs (reverse: .mcp.json → mcp/claude-code.json)
	for _, target := range KnownMCPTargets {
		if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(target.ProjectPath))); err == nil {
			add(target.ProjectPath, "mcp/"+target.RegistryFile)
		}
	}

	return files
}

// markdownFilesIn lists .md files directly inside dir, sorted.
func markdownFilesIn(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}
