package mount

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectToolDirs(t *testing.T) {
	dir := t.TempDir()

	// Create .claude and .opencode, but not .agents or .codex
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".opencode"), 0o755)

	found := DetectToolDirs(dir)
	if len(found) != 2 {
		t.Fatalf("DetectToolDirs() found %d dirs, want 2", len(found))
	}
	if found[0].Dir != ".claude" {
		t.Errorf("found[0].Dir = %q, want .claude", found[0].Dir)
	}
	if found[1].Dir != ".opencode" {
		t.Errorf("found[1].Dir = %q, want .opencode", found[1].Dir)
	}
}

func TestExpandToolTargets(t *testing.T) {
	toolDirs := []ToolDir{
		{Dir: ".claude", SkillsSubdir: "skills"},
		{Dir: ".opencode", SkillsSubdir: "skills"},
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

	// AGENTS.md: 1 (no expansion — not a skill)
	// code-review: 1 original + 2 tool dirs = 3
	// go-patterns: 1 original + 2 tool dirs = 3
	// Total: 7
	if len(expanded) != 7 {
		t.Fatalf("ExpandToolTargets() returned %d entries, want 7", len(expanded))
	}

	// Check that tool paths were created
	paths := make(map[string]bool)
	for _, e := range expanded {
		paths[e.RelativePath] = true
	}

	expected := []string{
		"AGENTS.md",
		"skills/code-review/SKILL.md",
		".claude/skills/code-review/SKILL.md",
		".opencode/skills/code-review/SKILL.md",
		"skills/go-patterns/SKILL.md",
		".claude/skills/go-patterns/SKILL.md",
		".opencode/skills/go-patterns/SKILL.md",
	}

	for _, path := range expected {
		if !paths[path] {
			t.Errorf("expected path %q not found in expanded entries", path)
		}
	}
}

func TestExpandToolTargetsNoDirs(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "skills/code-review/SKILL.md", FullPath: "/x"},
	}
	expanded := ExpandToolTargets(entries, nil)
	if len(expanded) != 1 {
		t.Fatalf("with no tool dirs, expected 1 entry, got %d", len(expanded))
	}
}
