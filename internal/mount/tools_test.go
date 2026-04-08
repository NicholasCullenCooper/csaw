package mount

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveToolDirsWithConfig(t *testing.T) {
	dir := t.TempDir()

	// Configure claude, auto-detect opencode
	os.MkdirAll(filepath.Join(dir, ".opencode"), 0o755)

	found := ResolveToolDirs(dir, []string{"claude"})

	names := make(map[string]bool)
	for _, d := range found {
		names[d.Dir] = true
	}
	for _, expected := range []string{".claude", ".opencode", ".agents"} {
		if !names[expected] {
			t.Errorf("expected %s in resolved dirs", expected)
		}
	}
}

func TestResolveToolDirsNoConfig(t *testing.T) {
	dir := t.TempDir()

	// No config, no existing dirs — only .agents fallback
	found := ResolveToolDirs(dir, nil)

	if len(found) != 1 {
		t.Fatalf("ResolveToolDirs() found %d dirs, want 1 (.agents)", len(found))
	}
	if found[0].Dir != ".agents" {
		t.Errorf("found[0].Dir = %q, want .agents", found[0].Dir)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents")); err != nil {
		t.Errorf(".agents directory was not created: %v", err)
	}
}

func TestResolveToolDirsAutoDetect(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755)

	found := ResolveToolDirs(dir, nil)

	names := make(map[string]bool)
	for _, d := range found {
		names[d.Dir] = true
	}
	if !names[".claude"] {
		t.Error("expected .claude to be auto-detected")
	}
	if !names[".cursor"] {
		t.Error("expected .cursor to be auto-detected")
	}
}

func TestExpandToolTargetsSkillsAndAgents(t *testing.T) {
	toolDirs := []ToolDir{
		{Dir: ".claude", SkillsSubdir: "skills", RulesSubdir: "rules"},
		{Dir: ".agents", SkillsSubdir: "skills"},
	}

	entries := []SourceEntry{
		{
			SourceName:    "dotagent",
			RelativePath:  "AGENTS.md",
			QualifiedPath: "dotagent/AGENTS.md",
			FullPath:      "/registry/AGENTS.md",
		},
		{
			SourceName:    "dotagent",
			RelativePath:  "agents/implementer.md",
			QualifiedPath: "dotagent/agents/implementer.md",
			FullPath:      "/registry/agents/implementer.md",
		},
		{
			SourceName:    "dotagent",
			RelativePath:  "skills/code-review/SKILL.md",
			QualifiedPath: "dotagent/skills/code-review/SKILL.md",
			FullPath:      "/registry/skills/code-review/SKILL.md",
		},
		{
			SourceName:    "dotagent",
			RelativePath:  "skills/go-patterns/SKILL.md",
			QualifiedPath: "dotagent/skills/go-patterns/SKILL.md",
			FullPath:      "/registry/skills/go-patterns/SKILL.md",
		},
	}

	expanded := ExpandToolTargets(entries, toolDirs)

	paths := make(map[string]bool)
	for _, e := range expanded {
		paths[e.RelativePath] = true
	}

	expectedPresent := []string{
		"AGENTS.md",                              // root file: kept at original path
		".claude/rules/implementer.md",           // agent: projected to .claude/rules/
		".claude/skills/code-review/SKILL.md",    // skill: projected to .claude/skills/
		".agents/skills/code-review/SKILL.md",    // skill: projected to .agents/skills/
		".claude/skills/go-patterns/SKILL.md",    // skill: projected to .claude/skills/
		".agents/skills/go-patterns/SKILL.md",    // skill: projected to .agents/skills/
	}
	for _, path := range expectedPresent {
		if !paths[path] {
			t.Errorf("expected path %q not found in expanded entries", path)
		}
	}

	// Agent and skill files should NOT be at original registry paths
	expectedAbsent := []string{
		"agents/implementer.md",
		"skills/code-review/SKILL.md",
		"skills/go-patterns/SKILL.md",
	}
	for _, path := range expectedAbsent {
		if paths[path] {
			t.Errorf("should not be at original path %q — should be projected to tool dirs", path)
		}
	}

	// AGENTS.md: 1 + agents/implementer.md → .claude/rules: 1 + 2 skills × 2 tool dirs = 6
	if len(expanded) != 6 {
		t.Fatalf("ExpandToolTargets() returned %d entries, want 6", len(expanded))
	}
}

func TestExpandMCPTargetsProjectsToToolPaths(t *testing.T) {
	entries := []SourceEntry{
		{
			SourceName:    "dotagent",
			RelativePath:  "mcp/claude-code.json",
			QualifiedPath: "dotagent/mcp/claude-code.json",
			FullPath:      "/registry/mcp/claude-code.json",
		},
		{
			SourceName:    "dotagent",
			RelativePath:  "mcp/vscode.json",
			QualifiedPath: "dotagent/mcp/vscode.json",
			FullPath:      "/registry/mcp/vscode.json",
		},
		{
			SourceName:    "dotagent",
			RelativePath:  "mcp/cursor.json",
			QualifiedPath: "dotagent/mcp/cursor.json",
			FullPath:      "/registry/mcp/cursor.json",
		},
		{
			SourceName:    "dotagent",
			RelativePath:  "AGENTS.md",
			QualifiedPath: "dotagent/AGENTS.md",
			FullPath:      "/registry/AGENTS.md",
		},
	}

	expanded := expandMCPTargets(entries)

	paths := make(map[string]bool)
	for _, e := range expanded {
		paths[e.RelativePath] = true
	}

	// MCP files should be projected to tool-specific paths
	expectedPresent := []string{
		".mcp.json",
		".vscode/mcp.json",
		".cursor/mcp.json",
		"AGENTS.md",
	}
	for _, path := range expectedPresent {
		if !paths[path] {
			t.Errorf("expected path %q not found in expanded entries", path)
		}
	}

	// MCP files should NOT remain at original registry path
	expectedAbsent := []string{
		"mcp/claude-code.json",
		"mcp/vscode.json",
		"mcp/cursor.json",
	}
	for _, path := range expectedAbsent {
		if paths[path] {
			t.Errorf("MCP config should not be at original path %q — should be projected", path)
		}
	}

	if len(expanded) != 4 {
		t.Fatalf("expandMCPTargets() returned %d entries, want 4", len(expanded))
	}
}

func TestExpandMCPTargetsUnknownFilePassesThrough(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "mcp/unknown-tool.json", FullPath: "/x"},
	}
	expanded := expandMCPTargets(entries)
	if len(expanded) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(expanded))
	}
	if expanded[0].RelativePath != "mcp/unknown-tool.json" {
		t.Errorf("unknown MCP file should keep original path, got %q", expanded[0].RelativePath)
	}
}

func TestExpandMCPTargetsNonJSONIgnored(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "mcp/README.md", FullPath: "/x"},
	}
	expanded := expandMCPTargets(entries)
	if expanded[0].RelativePath != "mcp/README.md" {
		t.Errorf("non-JSON in mcp/ should pass through, got %q", expanded[0].RelativePath)
	}
}

func TestExpandToolTargetsIncludesMCPProjection(t *testing.T) {
	toolDirs := []ToolDir{
		{Dir: ".claude", SkillsSubdir: "skills", RulesSubdir: "rules"},
	}

	entries := []SourceEntry{
		{RelativePath: "mcp/claude-code.json", FullPath: "/registry/mcp/claude-code.json"},
		{RelativePath: "skills/testing/SKILL.md", FullPath: "/registry/skills/testing/SKILL.md"},
	}

	expanded := ExpandToolTargets(entries, toolDirs)

	paths := make(map[string]bool)
	for _, e := range expanded {
		paths[e.RelativePath] = true
	}

	if !paths[".mcp.json"] {
		t.Error("MCP config should be projected to .mcp.json")
	}
	if !paths[".claude/skills/testing/SKILL.md"] {
		t.Error("skill should be projected to .claude/skills/testing/SKILL.md")
	}
}

func TestExpandToolTargetsNoToolDirsFallback(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "skills/code-review/SKILL.md", FullPath: "/x"},
	}
	expanded := ExpandToolTargets(entries, nil)
	if len(expanded) != 1 {
		t.Fatalf("with no tool dirs, expected 1 entry (original path fallback), got %d", len(expanded))
	}
	if expanded[0].RelativePath != "skills/code-review/SKILL.md" {
		t.Errorf("fallback should keep original path, got %q", expanded[0].RelativePath)
	}
}
