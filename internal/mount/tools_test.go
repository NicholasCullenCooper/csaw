package mount

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKindOfRegistryPaths(t *testing.T) {
	tests := []struct {
		path string
		want Kind
	}{
		{"AGENTS.md", KindInstruction},
		{"CLAUDE.md", KindInstruction},
		{"GEMINI.md", KindInstruction},
		{"QWEN.md", KindInstruction},
		{".goosehints", KindInstruction},
		{"agents/code-reviewer.md", KindAgent},
		{"agents/planner.md", KindAgent},
		{"skills/code-review/SKILL.md", KindSkill},
		{"skills/experimental/foo/SKILL.md", KindSkill},
		{"rules/go-conventions.md", KindRule},
		{"mcp/claude-code.json", KindMCP},
		{"hooks/pre-commit.sh", KindHook},
		{"hooks/on-save.json", KindHook},
		{"ignore/cursor", KindIgnore},
		{"ignore/cody", KindIgnore},
		{"unknown/random.txt", KindOther},
	}
	for _, tc := range tests {
		got := KindOf(SourceEntry{RelativePath: tc.path})
		if got != tc.want {
			t.Errorf("KindOf(%q) = %s, want %s", tc.path, got, tc.want)
		}
	}
}

func TestKindOfProjectPath(t *testing.T) {
	tests := []struct {
		path string
		want Kind
	}{
		{"AGENTS.md", KindInstruction},
		{"CLAUDE.md", KindInstruction},
		{".claude/agents/code-reviewer.md", KindAgent},
		{".opencode/agents/reviewer.md", KindAgent},
		// .cursor/agents/ and .codex/agents/ are NOT recognized — Cursor's agents
		// are runtime entities, Codex's agents are TOML inline tables.
		{".cursor/agents/planner.md", KindOther},
		{".codex/agents/foo.md", KindOther},
		{".claude/skills/code-review/SKILL.md", KindSkill},
		{".opencode/skills/foo/SKILL.md", KindSkill},
		{".agents/skills/foo/SKILL.md", KindSkill},
		{".codex/skills/setup/SKILL.md", KindSkill},
		{".claude/rules/go.md", KindRule},
		{".cursor/rules/style.md", KindRule},
		{".mcp.json", KindMCP},
		{".cursor/mcp.json", KindMCP},
		{".vscode/mcp.json", KindMCP},
		{".claude/hooks/pre-commit.sh", KindHook},
		{".cursorignore", KindIgnore},
		{".cody/ignore", KindIgnore},
		{".aiderignore", KindIgnore},
		{".tongyiignore", KindIgnore},
		{"GEMINI.md", KindInstruction},
		{".goosehints", KindInstruction},
		{"random/path.txt", KindOther},
	}
	for _, tc := range tests {
		got := KindOfProjectPath(tc.path)
		if got != tc.want {
			t.Errorf("KindOfProjectPath(%q) = %s, want %s", tc.path, got, tc.want)
		}
	}
}

func TestParseKind(t *testing.T) {
	tests := []struct {
		input   string
		want    Kind
		wantErr bool
	}{
		{"agents", KindAgent, false},
		{"agent", KindAgent, false},
		{"AGENTS", KindAgent, false},
		{"  skills ", KindSkill, false},
		{"rules", KindRule, false},
		{"mcp", KindMCP, false},
		{"instructions", KindInstruction, false},
		{"bogus", "", true},
	}
	for _, tc := range tests {
		got, err := ParseKind(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseKind(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ParseKind(%q) = %s, want %s", tc.input, got, tc.want)
		}
	}
}

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

func TestExpandToolTargetsAllProjections(t *testing.T) {
	toolDirs := []ToolDir{
		{Dir: ".claude", SkillsSubdir: "skills", RulesSubdir: "rules", AgentsSubdir: "agents"},
		{Dir: ".agents", SkillsSubdir: "skills"},
	}

	entries := []SourceEntry{
		{SourceName: "reg", RelativePath: "AGENTS.md", QualifiedPath: "reg/AGENTS.md", FullPath: "/reg/AGENTS.md"},
		{SourceName: "reg", RelativePath: "agents/reviewer.md", QualifiedPath: "reg/agents/reviewer.md", FullPath: "/reg/agents/reviewer.md"},
		{SourceName: "reg", RelativePath: "rules/go-conventions.md", QualifiedPath: "reg/rules/go-conventions.md", FullPath: "/reg/rules/go-conventions.md"},
		{SourceName: "reg", RelativePath: "skills/code-review/SKILL.md", QualifiedPath: "reg/skills/code-review/SKILL.md", FullPath: "/reg/skills/code-review/SKILL.md"},
	}

	expanded := ExpandToolTargets(entries, toolDirs)

	paths := make(map[string]bool)
	for _, e := range expanded {
		paths[e.RelativePath] = true
	}

	expectedPresent := []string{
		"AGENTS.md",                           // root file: kept
		".claude/agents/reviewer.md",          // subagent → .claude/agents/
		".claude/rules/go-conventions.md",     // rule → .claude/rules/
		".claude/skills/code-review/SKILL.md", // skill → .claude/skills/
		".agents/skills/code-review/SKILL.md", // skill → .agents/skills/
	}
	for _, path := range expectedPresent {
		if !paths[path] {
			t.Errorf("expected path %q not found in expanded entries", path)
		}
	}

	expectedAbsent := []string{
		"agents/reviewer.md",
		"rules/go-conventions.md",
		"skills/code-review/SKILL.md",
	}
	for _, path := range expectedAbsent {
		if paths[path] {
			t.Errorf("should not be at original path %q — should be projected", path)
		}
	}

	// AGENTS.md: 1 + agent→.claude: 1 + rule→.claude: 1 + skill×2 tools: 2 = 5
	if len(expanded) != 5 {
		t.Fatalf("ExpandToolTargets() returned %d entries, want 5", len(expanded))
	}
}

func TestExpandToolTargetsPreservesProtectedMetadata(t *testing.T) {
	toolDirs := []ToolDir{
		{Dir: ".claude", SkillsSubdir: "skills", RulesSubdir: "rules", AgentsSubdir: "agents"},
	}

	entries := []SourceEntry{
		{
			SourceName:    "team",
			RelativePath:  "agents/reviewer.md",
			QualifiedPath: "team/agents/reviewer.md",
			FullPath:      "/registry/agents/reviewer.md",
			Priority:      7,
			Protected:     true,
		},
	}

	expanded := ExpandToolTargets(entries, toolDirs)
	if got, want := len(expanded), 1; got != want {
		t.Fatalf("len(expanded) = %d, want %d", got, want)
	}
	if !expanded[0].Protected {
		t.Fatal("projected entry should remain protected")
	}
	if expanded[0].Priority != 7 {
		t.Fatalf("projected priority = %d, want 7", expanded[0].Priority)
	}
}

func TestScanAdoptableFiles(t *testing.T) {
	dir := t.TempDir()

	// Create various AI config files
	os.MkdirAll(filepath.Join(dir, ".claude", "skills", "testing"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".claude", "rules"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".claude", "agents"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".agents", "skills", "testing"), 0o755) // duplicate skill
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, ".claude", "skills", "testing", "SKILL.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, ".agents", "skills", "testing", "SKILL.md"), []byte("x"), 0o644) // same skill
	os.WriteFile(filepath.Join(dir, ".claude", "rules", "go.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, ".claude", "agents", "reviewer.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte("{}"), 0o644)

	files := ScanAdoptableFiles(dir)

	found := make(map[string]string)
	for _, f := range files {
		found[f.RegistryPath] = f.ProjectPath
	}

	expected := []string{
		"AGENTS.md",
		"skills/testing/SKILL.md",
		"rules/go.md",        // was agents/go.md, now rules/go.md
		"agents/reviewer.md", // subagent definition
		"mcp/claude-code.json",
	}
	for _, e := range expected {
		if _, ok := found[e]; !ok {
			t.Errorf("expected registry path %q not found (found: %v)", e, found)
		}
	}

	if len(files) != len(expected) {
		t.Errorf("len = %d, want %d (deduplication failed?)", len(files), len(expected))
	}
}

func TestScanAdoptableFilesEmpty(t *testing.T) {
	dir := t.TempDir()
	files := ScanAdoptableFiles(dir)
	if len(files) != 0 {
		t.Fatalf("expected 0 adoptable files, got %d", len(files))
	}
}

func TestExpandIgnoreTargetsProjectsToToolPaths(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "reg", RelativePath: "ignore/cursor", FullPath: "/r/ignore/cursor"},
		{SourceName: "reg", RelativePath: "ignore/cody", FullPath: "/r/ignore/cody"},
		{SourceName: "reg", RelativePath: "ignore/aider", FullPath: "/r/ignore/aider"},
		{SourceName: "reg", RelativePath: "ignore/lingma", FullPath: "/r/ignore/lingma"},
		{SourceName: "reg", RelativePath: "ignore/unknown-tool", FullPath: "/r/ignore/unknown-tool"},
	}

	expanded := expandIgnoreTargets(entries)
	paths := make(map[string]bool)
	for _, e := range expanded {
		paths[e.RelativePath] = true
	}

	for _, path := range []string{".cursorignore", ".cody/ignore", ".aiderignore", ".tongyiignore"} {
		if !paths[path] {
			t.Errorf("expected ignore projection %q not found", path)
		}
	}
	// Unknown ignore filename should pass through at original path
	if !paths["ignore/unknown-tool"] {
		t.Error("unknown ignore file should pass through at original path")
	}
}

func TestExpandToolTargetsProjectsHooks(t *testing.T) {
	// Use a synthetic second tool to verify multi-tool hook expansion;
	// claude is the only in-registry tool with HooksSubdir today.
	toolDirs := []ToolDir{
		{Dir: ".claude", HooksSubdir: "hooks"},
		{Dir: ".faketool", HooksSubdir: "hooks"},
	}
	entries := []SourceEntry{
		{SourceName: "reg", RelativePath: "hooks/pre-commit.sh", FullPath: "/r/hooks/pre-commit.sh"},
	}
	expanded := ExpandToolTargets(entries, toolDirs)
	paths := make(map[string]bool)
	for _, e := range expanded {
		paths[e.RelativePath] = true
	}
	for _, path := range []string{".claude/hooks/pre-commit.sh", ".faketool/hooks/pre-commit.sh"} {
		if !paths[path] {
			t.Errorf("expected hook projection %q not found", path)
		}
	}
	// Hook entries should NOT remain at original registry path when projected
	if paths["hooks/pre-commit.sh"] {
		t.Error("hook entry should not remain at original path when tools have HooksSubdir")
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
		{Dir: ".claude", SkillsSubdir: "skills", RulesSubdir: "rules", AgentsSubdir: "agents"},
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
