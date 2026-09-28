package mount

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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
		{"skills/deploy/references/runbook.md", KindSkill},
		{"skills/deploy/scripts/run.sh", KindSkill},
		{"skills/experimental/foo/SKILL.md", KindSkill},
		{"skills/README.md", KindOther},
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
		{".goosehints", KindInstruction},
		{".claude/agents/code-reviewer.md", KindAgent},
		{".claude/skills/code-review/SKILL.md", KindSkill},
		{".claude/skills/deploy/scripts/run.sh", KindSkill},
		{".agents/skills/foo/SKILL.md", KindSkill},
		{".claude/rules/go.md", KindRule},
		{".mcp.json", KindMCP},
		{".cursor/mcp.json", KindMCP},
		{".vscode/mcp.json", KindMCP},
		{".claude/hooks/pre-commit.sh", KindHook},
		{".cursorignore", KindIgnore},
		{".cody/ignore", KindIgnore},
		{".aiderignore", KindIgnore},
		{".tongyiignore", KindIgnore},
		{".geminiignore", KindIgnore},
		// Directories of tools csaw does not support are not classified.
		{".cursor/rules/style.mdc", KindOther},
		{".github/agents/reviewer.agent.md", KindOther},
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
		{"hooks", KindHook, false},
		{"ignore", KindIgnore, false},
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

func toolNames(tools []ToolSpec) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func specs(names ...string) []ToolSpec {
	out := make([]ToolSpec, 0, len(names))
	for _, name := range names {
		spec, ok := ToolRegistry[name]
		if !ok {
			panic("unknown tool in test: " + name)
		}
		out = append(out, spec)
	}
	return out
}

func TestToolRegistryIsClaudeAndCodex(t *testing.T) {
	if got, want := AllToolNames(), []string{"claude", "codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AllToolNames() = %v, want %v (other tools live in the JSON catalogue only)", got, want)
	}
}

func TestResolveToolsWithConfig(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".codex"), 0o755)

	got := toolNames(ResolveTools(dir, []string{"claude"}))
	want := []string{"claude", "codex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveTools() = %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents")); err == nil {
		t.Error(".agents must not be created when real tools are resolved")
	}
}

func TestResolveToolsNoConfigFallsBackToNeutral(t *testing.T) {
	dir := t.TempDir()

	got := ResolveTools(dir, nil)
	if len(got) != 1 || got[0].Name != StandardFallback.Name {
		t.Fatalf("ResolveTools() = %v, want only the neutral fallback", toolNames(got))
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents")); err == nil {
		t.Error("ResolveTools must not create directories")
	}
}

func TestResolveToolsAutoDetect(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)

	got := toolNames(ResolveTools(dir, nil))
	if want := []string{"claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveTools() = %v, want %v", got, want)
	}

	os.MkdirAll(filepath.Join(dir, ".codex"), 0o755)
	got = toolNames(ResolveTools(dir, nil))
	if want := []string{"claude", "codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveTools() after .codex = %v, want %v", got, want)
	}
}

func TestResolveToolsDedupesAndIgnoresUnknown(t *testing.T) {
	dir := t.TempDir()
	got := toolNames(ResolveTools(dir, []string{"codex", "claude", "codex", "cursor"}))
	want := []string{"claude", "codex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveTools() = %v, want %v", got, want)
	}
}

func TestToolRegistryInvariants(t *testing.T) {
	check := func(name string, spec ToolSpec) {
		if spec.DisplayName == "" || spec.InstructionFile == "" {
			t.Errorf("%s: DisplayName and InstructionFile are required", name)
		}
		if spec.InstructionFile != "AGENTS.md" && spec.BridgeImport == "" {
			t.Errorf("%s: reads %s, so it needs a BridgeImport", name, spec.InstructionFile)
		}
		for kind, dir := range spec.Writes {
			if !spec.readsDir(kind, dir) {
				t.Errorf("%s/%s: write target %q must appear in Reads", name, kind, dir)
			}
		}
		for kind := range spec.Reads {
			if _, ok := spec.Writes[kind]; !ok {
				t.Errorf("%s/%s: tool reads the kind but has no write target", name, kind)
			}
		}
	}
	for name, spec := range ToolRegistry {
		if spec.Name != name {
			t.Errorf("%s: Name field %q must match registry key", name, spec.Name)
		}
		check(name, spec)
	}
	check("fallback", StandardFallback)
}

func TestPlanTargetsForSupportedTools(t *testing.T) {
	tests := []struct {
		name  string
		tools []string
		kind  Kind
		want  []string
	}{
		{"claude skills", []string{"claude"}, KindSkill, []string{".claude/skills"}},
		{"codex skills", []string{"codex"}, KindSkill, []string{".agents/skills"}},
		// Claude Code does not read .agents/, so both directories are needed.
		{"claude+codex skills", []string{"claude", "codex"}, KindSkill, []string{".agents/skills", ".claude/skills"}},
		{"claude+codex agents", []string{"claude", "codex"}, KindAgent, []string{".claude/agents"}},
		{"claude+codex rules", []string{"claude", "codex"}, KindRule, []string{".claude/rules"}},
		{"claude+codex hooks", []string{"claude", "codex"}, KindHook, []string{".claude/hooks"}},
		{"codex rules (unsupported)", []string{"codex"}, KindRule, nil},
		{"codex agents (unsupported)", []string{"codex"}, KindAgent, nil},
	}
	for _, tc := range tests {
		got := PlanTargets(tc.kind, specs(tc.tools...))
		if len(got) == 0 {
			got = nil
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: PlanTargets = %v, want %v", tc.name, got, tc.want)
		}
	}

	if got := PlanTargets(KindSkill, []ToolSpec{StandardFallback}); !reflect.DeepEqual(got, []string{".agents/skills"}) {
		t.Errorf("fallback skills = %v, want [.agents/skills]", got)
	}
}

// TestPlanTargetsSetCover exercises the greedy cover with synthetic specs so
// the algorithm stays correct when compatibility paths are added later: one
// shared directory should serve every tool that reads it.
func TestPlanTargetsSetCover(t *testing.T) {
	native := ToolSpec{Name: "native", Reads: map[Kind][]string{KindSkill: {".native/skills"}}, Writes: map[Kind]string{KindSkill: ".native/skills"}}
	compat1 := ToolSpec{Name: "compat1", Reads: map[Kind][]string{KindSkill: {".compat1/skills", ".native/skills"}}, Writes: map[Kind]string{KindSkill: ".compat1/skills"}}
	compat2 := ToolSpec{Name: "compat2", Reads: map[Kind][]string{KindSkill: {".compat2/skills", ".native/skills"}}, Writes: map[Kind]string{KindSkill: ".compat2/skills"}}
	loner := ToolSpec{Name: "loner", Reads: map[Kind][]string{KindSkill: {".loner/skills"}}, Writes: map[Kind]string{KindSkill: ".loner/skills"}}
	blind := ToolSpec{Name: "blind"}

	got := PlanTargets(KindSkill, []ToolSpec{compat1, compat2, native, loner, blind})
	want := []string{".loner/skills", ".native/skills"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PlanTargets = %v, want %v", got, want)
	}

	// Without the shared native directory, each compat tool gets its own.
	got = PlanTargets(KindSkill, []ToolSpec{compat1, compat2})
	want = []string{".compat1/skills", ".compat2/skills"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PlanTargets without shared dir = %v, want %v", got, want)
	}
}

func relPaths(entries []SourceEntry) map[string]SourceEntry {
	paths := make(map[string]SourceEntry, len(entries))
	for _, e := range entries {
		paths[e.RelativePath] = e
	}
	return paths
}

func sortedPaths(paths map[string]SourceEntry) []string {
	out := make([]string, 0, len(paths))
	for p := range paths {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func TestExpandToolTargetsAllProjections(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "reg", RelativePath: "AGENTS.md", QualifiedPath: "reg/AGENTS.md", FullPath: "/reg/AGENTS.md"},
		{SourceName: "reg", RelativePath: "agents/reviewer.md", QualifiedPath: "reg/agents/reviewer.md", FullPath: "/reg/agents/reviewer.md"},
		{SourceName: "reg", RelativePath: "rules/go-conventions.md", QualifiedPath: "reg/rules/go-conventions.md", FullPath: "/reg/rules/go-conventions.md"},
		{SourceName: "reg", RelativePath: "skills/code-review/SKILL.md", QualifiedPath: "reg/skills/code-review/SKILL.md", FullPath: "/reg/skills/code-review/SKILL.md"},
		{SourceName: "reg", RelativePath: "hooks/pre-commit.sh", QualifiedPath: "reg/hooks/pre-commit.sh", FullPath: "/reg/hooks/pre-commit.sh"},
	}

	expanded := ExpandToolTargets(entries, specs("claude", "codex"))
	paths := relPaths(expanded)

	for _, want := range []string{
		"AGENTS.md",                           // root file: kept, Codex reads it
		"CLAUDE.md",                           // generated bridge for Claude Code
		".claude/agents/reviewer.md",          // subagent → .claude/agents/
		".claude/rules/go-conventions.md",     // rule → .claude/rules/
		".claude/skills/code-review/SKILL.md", // skill → .claude/skills/
		".agents/skills/code-review/SKILL.md", // skill → .agents/skills/ for Codex
		".claude/hooks/pre-commit.sh",         // hook script → .claude/hooks/
	} {
		if _, ok := paths[want]; !ok {
			t.Errorf("expected path %q not found in %v", want, sortedPaths(paths))
		}
	}
	for _, gone := range []string{"agents/reviewer.md", "rules/go-conventions.md", "skills/code-review/SKILL.md", "hooks/pre-commit.sh", ".codex/skills/code-review/SKILL.md"} {
		if _, ok := paths[gone]; ok {
			t.Errorf("path %q should not be present", gone)
		}
	}
	if len(expanded) != 7 {
		t.Fatalf("ExpandToolTargets() returned %d entries, want 7: %v", len(expanded), sortedPaths(paths))
	}
}

func TestExpandToolTargetsProjectsWholeSkillDirectory(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "reg", RelativePath: "skills/deploy/SKILL.md", QualifiedPath: "reg/skills/deploy/SKILL.md", FullPath: "/reg/skills/deploy/SKILL.md"},
		{SourceName: "reg", RelativePath: "skills/deploy/references/runbook.md", QualifiedPath: "reg/skills/deploy/references/runbook.md", FullPath: "/reg/skills/deploy/references/runbook.md"},
		{SourceName: "reg", RelativePath: "skills/deploy/scripts/deploy.sh", QualifiedPath: "reg/skills/deploy/scripts/deploy.sh", FullPath: "/reg/skills/deploy/scripts/deploy.sh"},
		{SourceName: "reg", RelativePath: "skills/README.md", QualifiedPath: "reg/skills/README.md", FullPath: "/reg/skills/README.md"},
	}

	paths := relPaths(ExpandToolTargets(entries, specs("claude")))
	for _, want := range []string{
		".claude/skills/deploy/SKILL.md",
		".claude/skills/deploy/references/runbook.md",
		".claude/skills/deploy/scripts/deploy.sh",
		"skills/README.md", // not inside a skill directory: kept as-is
	} {
		if _, ok := paths[want]; !ok {
			t.Errorf("expected %q in %v", want, sortedPaths(paths))
		}
	}
	for p := range paths {
		if strings.HasPrefix(p, "skills/deploy/") {
			t.Errorf("supporting file %q must not land at the project root", p)
		}
	}
}

func TestExpandToolTargetsExperimentalSkillKeepsName(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "reg", RelativePath: "skills/experimental/new-idea/SKILL.md", FullPath: "/reg/skills/experimental/new-idea/SKILL.md"},
	}
	paths := relPaths(ExpandToolTargets(entries, specs("codex")))
	if _, ok := paths[".agents/skills/new-idea/SKILL.md"]; !ok {
		t.Fatalf("experimental skill should project under its own name, got %v", sortedPaths(paths))
	}
}

func TestExpandToolTargetsGeneratesClaudeBridge(t *testing.T) {
	agents := SourceEntry{SourceName: "team", RelativePath: "AGENTS.md", QualifiedPath: "team/AGENTS.md", FullPath: "/team/AGENTS.md", Priority: 50, Protected: true}

	t.Run("claude gets CLAUDE.md", func(t *testing.T) {
		paths := relPaths(ExpandToolTargets([]SourceEntry{agents}, specs("claude", "codex")))
		bridge, ok := paths["CLAUDE.md"]
		if !ok {
			t.Fatalf("CLAUDE.md bridge missing: %v", sortedPaths(paths))
		}
		if !bridge.Generated || !strings.Contains(bridge.Content, "@AGENTS.md") {
			t.Errorf("bridge should be generated and import AGENTS.md: %+v", bridge)
		}
		if bridge.SourceName != "team" || bridge.Priority != 50 || !bridge.Protected {
			t.Errorf("bridge should inherit source metadata: %+v", bridge)
		}
		if bridge.FullPath != agents.FullPath {
			t.Errorf("bridge FullPath = %q, want the AGENTS.md path for provenance", bridge.FullPath)
		}
	})

	t.Run("no bridge when the registry ships CLAUDE.md", func(t *testing.T) {
		claude := SourceEntry{SourceName: "team", RelativePath: "CLAUDE.md", FullPath: "/team/CLAUDE.md"}
		paths := relPaths(ExpandToolTargets([]SourceEntry{agents, claude}, specs("claude")))
		if paths["CLAUDE.md"].Generated {
			t.Error("registry CLAUDE.md must win over the generated bridge")
		}
	})

	t.Run("no bridge without AGENTS.md", func(t *testing.T) {
		rule := SourceEntry{SourceName: "team", RelativePath: "rules/go.md", FullPath: "/team/rules/go.md"}
		paths := relPaths(ExpandToolTargets([]SourceEntry{rule}, specs("claude")))
		if _, ok := paths["CLAUDE.md"]; ok {
			t.Error("nothing to bridge without AGENTS.md")
		}
	})

	t.Run("codex alone gets no bridge", func(t *testing.T) {
		paths := relPaths(ExpandToolTargets([]SourceEntry{agents}, specs("codex")))
		if len(paths) != 1 {
			t.Errorf("expected only AGENTS.md, got %v", sortedPaths(paths))
		}
	})
}

func TestExpandToolTargetsPreservesProtectedMetadata(t *testing.T) {
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

	expanded := ExpandToolTargets(entries, specs("claude"))
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

func TestExpandToolTargetsKeepsUnsupportedKindsAtRegistryPath(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "reg", RelativePath: "rules/go.md", FullPath: "/r/rules/go.md"},
		{SourceName: "reg", RelativePath: "hooks/pre-commit.sh", FullPath: "/r/hooks/pre-commit.sh"},
	}
	// Codex has no rules or hooks directory, so with Codex alone the files
	// stay where the registry put them.
	paths := relPaths(ExpandToolTargets(entries, specs("codex")))
	for _, want := range []string{"rules/go.md", "hooks/pre-commit.sh"} {
		if _, ok := paths[want]; !ok {
			t.Errorf("expected %q to stay at its registry path, got %v", want, sortedPaths(paths))
		}
	}
}

func TestScanAdoptableFiles(t *testing.T) {
	dir := t.TempDir()

	mk := func(rel, content string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}
	mk("AGENTS.md", "x")
	mk(".claude/skills/testing/SKILL.md", "x")
	mk(".claude/skills/testing/references/notes.md", "x")
	mk(".agents/skills/testing/SKILL.md", "x") // same skill via the neutral dir
	mk(".claude/rules/go.md", "x")
	mk(".claude/agents/reviewer.md", "x")
	mk(".mcp.json", "{}")

	files := ScanAdoptableFiles(dir)

	found := make(map[string]string)
	for _, f := range files {
		found[f.RegistryPath] = f.ProjectPath
	}

	// Directories are scanned in sorted order, so the neutral .agents/ copy of
	// SKILL.md wins the dedupe while its references/ file is only under .claude/.
	expected := map[string]string{
		"AGENTS.md":                          "AGENTS.md",
		"skills/testing/SKILL.md":            ".agents/skills/testing/SKILL.md",
		"skills/testing/references/notes.md": ".claude/skills/testing/references/notes.md",
		"rules/go.md":                        ".claude/rules/go.md",
		"agents/reviewer.md":                 ".claude/agents/reviewer.md",
		"mcp/claude-code.json":               ".mcp.json",
	}
	for registryPath, projectPath := range expected {
		got, ok := found[registryPath]
		if !ok {
			t.Errorf("expected registry path %q not found (found: %v)", registryPath, found)
			continue
		}
		if got != projectPath {
			t.Errorf("%s adopted from %q, want %q", registryPath, got, projectPath)
		}
	}
	if len(files) != len(expected) {
		t.Errorf("len = %d, want %d (deduplication failed?): %v", len(files), len(expected), found)
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
		{SourceName: "reg", RelativePath: "ignore/gemini", FullPath: "/r/ignore/gemini"},
		{SourceName: "reg", RelativePath: "ignore/unknown-tool", FullPath: "/r/ignore/unknown-tool"},
	}

	paths := relPaths(expandIgnoreTargets(entries))
	for _, path := range []string{".cursorignore", ".cody/ignore", ".aiderignore", ".tongyiignore", ".geminiignore"} {
		if _, ok := paths[path]; !ok {
			t.Errorf("expected ignore projection %q not found", path)
		}
	}
	if _, ok := paths["ignore/unknown-tool"]; !ok {
		t.Error("unknown ignore file should pass through at original path")
	}
}

func TestExpandMCPTargetsProjectsToToolPaths(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "dotagent", RelativePath: "mcp/claude-code.json", QualifiedPath: "dotagent/mcp/claude-code.json", FullPath: "/registry/mcp/claude-code.json"},
		{SourceName: "dotagent", RelativePath: "mcp/vscode.json", QualifiedPath: "dotagent/mcp/vscode.json", FullPath: "/registry/mcp/vscode.json"},
		{SourceName: "dotagent", RelativePath: "mcp/cursor.json", QualifiedPath: "dotagent/mcp/cursor.json", FullPath: "/registry/mcp/cursor.json"},
		{SourceName: "dotagent", RelativePath: "AGENTS.md", QualifiedPath: "dotagent/AGENTS.md", FullPath: "/registry/AGENTS.md"},
	}

	expanded := expandMCPTargets(entries)
	paths := relPaths(expanded)

	for _, path := range []string{".mcp.json", ".vscode/mcp.json", ".cursor/mcp.json", "AGENTS.md"} {
		if _, ok := paths[path]; !ok {
			t.Errorf("expected path %q not found in expanded entries", path)
		}
	}
	for _, path := range []string{"mcp/claude-code.json", "mcp/vscode.json", "mcp/cursor.json"} {
		if _, ok := paths[path]; ok {
			t.Errorf("MCP config should not be at original path %q — should be projected", path)
		}
	}
	if len(expanded) != 4 {
		t.Fatalf("expandMCPTargets() returned %d entries, want 4", len(expanded))
	}
}

func TestExpandMCPTargetsUnknownFilePassesThrough(t *testing.T) {
	entries := []SourceEntry{{RelativePath: "mcp/unknown-tool.json", FullPath: "/x"}}
	expanded := expandMCPTargets(entries)
	if len(expanded) != 1 || expanded[0].RelativePath != "mcp/unknown-tool.json" {
		t.Errorf("unknown MCP file should keep original path, got %+v", expanded)
	}
}

func TestExpandMCPTargetsNonJSONIgnored(t *testing.T) {
	entries := []SourceEntry{{RelativePath: "mcp/README.md", FullPath: "/x"}}
	expanded := expandMCPTargets(entries)
	if expanded[0].RelativePath != "mcp/README.md" {
		t.Errorf("non-JSON in mcp/ should pass through, got %q", expanded[0].RelativePath)
	}
}

func TestExpandToolTargetsIncludesMCPProjection(t *testing.T) {
	entries := []SourceEntry{
		{RelativePath: "mcp/claude-code.json", FullPath: "/registry/mcp/claude-code.json"},
		{RelativePath: "skills/testing/SKILL.md", FullPath: "/registry/skills/testing/SKILL.md"},
	}
	paths := relPaths(ExpandToolTargets(entries, specs("claude")))
	if _, ok := paths[".mcp.json"]; !ok {
		t.Error("MCP config should be projected to .mcp.json")
	}
	if _, ok := paths[".claude/skills/testing/SKILL.md"]; !ok {
		t.Error("skill should be projected to .claude/skills/testing/SKILL.md")
	}
}

func TestExpandToolTargetsNoToolsFallback(t *testing.T) {
	entries := []SourceEntry{{RelativePath: "skills/code-review/SKILL.md", FullPath: "/x"}}
	expanded := ExpandToolTargets(entries, nil)
	if len(expanded) != 1 || expanded[0].RelativePath != "skills/code-review/SKILL.md" {
		t.Fatalf("with no tools, expected the original path, got %+v", expanded)
	}
}

func TestSkillParts(t *testing.T) {
	tests := []struct {
		rel      string
		wantName string
		wantRest string
		wantOK   bool
	}{
		{"skills/deploy/SKILL.md", "deploy", "SKILL.md", true},
		{"skills/deploy/references/a.md", "deploy", "references/a.md", true},
		{"skills/experimental/new/SKILL.md", "new", "SKILL.md", true},
		{"team/skills/deploy/SKILL.md", "deploy", "SKILL.md", true},
		{"skills/README.md", "", "", false},
		{"skills/experimental/README.md", "", "", false},
		{"rules/go.md", "", "", false},
	}
	for _, tc := range tests {
		name, rest, ok := skillParts(tc.rel)
		if name != tc.wantName || rest != tc.wantRest || ok != tc.wantOK {
			t.Errorf("skillParts(%q) = (%q, %q, %v), want (%q, %q, %v)", tc.rel, name, rest, ok, tc.wantName, tc.wantRest, tc.wantOK)
		}
	}
}

func TestKnownTopLevelDirs(t *testing.T) {
	got := KnownTopLevelDirs()
	if want := []string{".agents", ".claude"}; !reflect.DeepEqual(got, want) {
		t.Errorf("KnownTopLevelDirs() = %v, want %v", got, want)
	}
}
