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
