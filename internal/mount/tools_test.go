package mount

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectToolDirsWithExisting(t *testing.T) {
	dir := t.TempDir()

	// Create .claude and .opencode, but not .codex
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".opencode"), 0o755)

	found := DetectToolDirs(dir)

	// Should find .claude, .opencode, and always include .agents (created as fallback)
	if len(found) != 3 {
		names := make([]string, len(found))
		for i, d := range found {
			names[i] = d.Dir
		}
		t.Fatalf("DetectToolDirs() found %d dirs %v, want 3 (.claude, .opencode, .agents)", len(found), names)
	}
}

func TestDetectToolDirsAlwaysIncludesAgents(t *testing.T) {
	dir := t.TempDir()

	// No tool directories exist — .agents should be created
	found := DetectToolDirs(dir)

	if len(found) != 1 {
		t.Fatalf("DetectToolDirs() found %d dirs, want 1 (.agents)", len(found))
	}
	if found[0].Dir != ".agents" {
		t.Errorf("found[0].Dir = %q, want .agents", found[0].Dir)
	}

	// Verify .agents was created on disk
	if _, err := os.Stat(filepath.Join(dir, ".agents")); err != nil {
		t.Errorf(".agents directory was not created: %v", err)
	}
}

func TestExpandToolTargetsSkillsGoToToolDirsOnly(t *testing.T) {
	toolDirs := []ToolDir{
		{Dir: ".claude", SkillsSubdir: "skills"},
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

	// Non-skill files: kept at original paths
	expectedPresent := []string{
		"AGENTS.md",
		"agents/implementer.md",
		".claude/skills/code-review/SKILL.md",
		".agents/skills/code-review/SKILL.md",
		".claude/skills/go-patterns/SKILL.md",
		".agents/skills/go-patterns/SKILL.md",
	}
	for _, path := range expectedPresent {
		if !paths[path] {
			t.Errorf("expected path %q not found in expanded entries", path)
		}
	}

	// Skill files: NOT at original registry path
	expectedAbsent := []string{
		"skills/code-review/SKILL.md",
		"skills/go-patterns/SKILL.md",
	}
	for _, path := range expectedAbsent {
		if paths[path] {
			t.Errorf("skill should not be at original path %q — should only be in tool dirs", path)
		}
	}

	// AGENTS.md: 1 + agents/implementer.md: 1 + 2 skills × 2 tool dirs = 6
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
		{Dir: ".claude", SkillsSubdir: "skills"},
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
