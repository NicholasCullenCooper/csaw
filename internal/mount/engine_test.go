package mount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

type staticResolver struct {
	action ConflictAction
}

func (r staticResolver) Resolve(conflict Conflict) (ConflictAction, error) {
	_ = conflict
	return r.action, nil
}

func TestApplyUnmountAndRestoreState(t *testing.T) {
	if canSymlink, reason := canCreateSymlink(); !canSymlink {
		t.Skip(reason)
	}

	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))
	project := filepath.Join(root, "project")
	sourceRoot := filepath.Join(root, "source")

	if err := os.MkdirAll(filepath.Join(project, ".git", "info"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceRoot, "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	sourceFile := filepath.Join(sourceRoot, "agents", "base.md")
	if err := os.WriteFile(sourceFile, []byte("source"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	targetFile := filepath.Join(project, "agents", "base.md")
	if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("local"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := Apply(project, paths, []SourceEntry{{
		SourceName:    "personal",
		RelativePath:  "agents/base.md",
		QualifiedPath: "personal/agents/base.md",
		FullPath:      sourceFile,
	}}, staticResolver{action: ConflictOverwrite})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Linked != 1 || result.Stashed != 1 {
		t.Fatalf("Apply() result = %#v", result)
	}

	if _, err := os.Lstat(targetFile); err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if !linkmode.IsLink(linkmode.Detect(), targetFile, sourceFile) {
		t.Fatal("mounted target is not a csaw-managed link")
	}

	restoreState, err := workspace.ReadRestoreState(paths, project)
	if err != nil {
		t.Fatalf("ReadRestoreState() error = %v", err)
	}
	if len(restoreState.Entries) != 1 {
		t.Fatalf("restore state = %#v, want 1 entry", restoreState)
	}

	unmountResult, err := Unmount(project, Selection{})
	if err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
	if unmountResult.Removed != 1 || unmountResult.Restored != 1 {
		t.Fatalf("Unmount() result = %#v", unmountResult)
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "local" {
		t.Fatalf("restored content = %q, want %q", string(content), "local")
	}
}

func TestApplyRecordsProtectedSourceHash(t *testing.T) {
	if canSymlink, reason := canCreateSymlink(); !canSymlink {
		t.Skip(reason)
	}

	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))
	project := filepath.Join(root, "project")
	sourceRoot := filepath.Join(root, "source")

	if err := os.MkdirAll(filepath.Join(project, ".git", "info"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}

	sourceFile := filepath.Join(sourceRoot, "AGENTS.md")
	if err := os.WriteFile(sourceFile, []byte("approved"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	wantHash, err := workspace.FileSHA256(sourceFile)
	if err != nil {
		t.Fatalf("FileSHA256() error = %v", err)
	}

	_, err = Apply(project, paths, []SourceEntry{{
		SourceName:    "team",
		RelativePath:  "AGENTS.md",
		QualifiedPath: "team/AGENTS.md",
		FullPath:      sourceFile,
		Protected:     true,
	}}, staticResolver{action: ConflictOverwrite})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	state, err := workspace.ReadMountState(project)
	if err != nil {
		t.Fatalf("ReadMountState() error = %v", err)
	}
	if got, want := len(state.Entries), 1; got != want {
		t.Fatalf("len(state.Entries) = %d, want %d", got, want)
	}
	if !state.Entries[0].Protected {
		t.Fatal("mounted state entry should be marked protected")
	}
	if state.Entries[0].SourceSHA256 != wantHash {
		t.Fatalf("SourceSHA256 = %q, want %q", state.Entries[0].SourceSHA256, wantHash)
	}
}

func TestIgnorePatternsFilterEntries(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "skills", "experimental"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, runtime.IgnoreFile), []byte("skills/experimental/**\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "skills", "experimental", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "skills", "stable.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	entries, err := EnumerateSourceEntries(sources.CatalogSource{Name: "personal", Root: sourceRoot})
	if err != nil {
		t.Fatalf("EnumerateSourceEntries() error = %v", err)
	}
	patterns, err := ReadIgnorePatterns(sourceRoot)
	if err != nil {
		t.Fatalf("ReadIgnorePatterns() error = %v", err)
	}
	filtered, err := ApplyIgnore(entries, patterns)
	if err != nil {
		t.Fatalf("ApplyIgnore() error = %v", err)
	}
	if got, want := len(filtered), 1; got != want {
		t.Fatalf("len(filtered) = %d, want %d (%v)", got, want, filtered)
	}
}

func TestResolveConflictsByPriorityHigherWins(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "team", RelativePath: "AGENTS.md", Priority: 0},
		{SourceName: "personal", RelativePath: "AGENTS.md", Priority: 10},
	}
	resolved, err := resolveConflictsByPriority(entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("len = %d, want 1", len(resolved))
	}
	if resolved[0].SourceName != "personal" {
		t.Fatalf("winner = %q, want personal", resolved[0].SourceName)
	}
}

func TestResolveConflictsByPriorityTiedErrors(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "team", RelativePath: "AGENTS.md", Priority: 0},
		{SourceName: "community", RelativePath: "AGENTS.md", Priority: 0},
	}
	_, err := resolveConflictsByPriority(entries)
	if err == nil {
		t.Fatal("expected error for tied priority")
	}
}

func TestResolveConflictsProtectedWinsOverPriority(t *testing.T) {
	// Even with higher priority on personal, the team's protected file wins.
	entries := []SourceEntry{
		{SourceName: "team", RelativePath: "AGENTS.md", Priority: 0, Protected: true},
		{SourceName: "personal", RelativePath: "AGENTS.md", Priority: 100},
	}
	resolved, err := resolveConflictsByPriority(entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 1 || resolved[0].SourceName != "team" {
		t.Fatalf("protected team file should win; got %v", resolved)
	}
}

func TestResolveConflictsTwoSourcesProtectSamePathErrors(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "team", RelativePath: "AGENTS.md", Protected: true},
		{SourceName: "community", RelativePath: "AGENTS.md", Protected: true},
	}
	_, err := resolveConflictsByPriority(entries)
	if err == nil {
		t.Fatal("expected protection conflict error")
	}
}

func TestResolveConflictsByPriorityNoConflict(t *testing.T) {
	entries := []SourceEntry{
		{SourceName: "team", RelativePath: "AGENTS.md", Priority: 0},
		{SourceName: "personal", RelativePath: "skills/foo/SKILL.md", Priority: 10},
	}
	resolved, err := resolveConflictsByPriority(entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("len = %d, want 2", len(resolved))
	}
}

func canCreateSymlink() (bool, string) {
	root, err := os.MkdirTemp("", "csaw-symlink-*")
	if err != nil {
		return false, err.Error()
	}
	defer os.RemoveAll(root)
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		return false, err.Error()
	}
	if err := os.Symlink(target, link); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func generatedBridgeFixture(t *testing.T) (project string, paths runtime.Paths, entry SourceEntry) {
	t.Helper()
	root := t.TempDir()
	paths = runtime.BuildPaths(filepath.Join(root, ".csaw"))
	project = filepath.Join(root, "project")
	sourceRoot := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(project, ".git", "info"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	agents := filepath.Join(sourceRoot, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("# instructions\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	entry = SourceEntry{
		SourceName:    "team",
		RelativePath:  "CLAUDE.md",
		QualifiedPath: "team/AGENTS.md→CLAUDE.md",
		FullPath:      agents,
		Generated:     true,
		Content:       "@AGENTS.md\n",
	}
	return project, paths, entry
}

func TestApplyWritesAndUnmountsGeneratedFiles(t *testing.T) {
	project, paths, entry := generatedBridgeFixture(t)
	target := filepath.Join(project, "CLAUDE.md")

	result, err := Apply(project, paths, []SourceEntry{entry}, staticResolver{action: ConflictOverwrite})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Linked != 1 {
		t.Fatalf("Apply() result = %#v, want 1 written", result)
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatal("generated file must be a regular file, not a link")
	}
	content, _ := os.ReadFile(target)
	if string(content) != entry.Content {
		t.Fatalf("generated content = %q, want %q", content, entry.Content)
	}

	state, err := workspace.ReadMountState(project)
	if err != nil {
		t.Fatalf("ReadMountState() error = %v", err)
	}
	if len(state.Entries) != 1 || !state.Entries[0].Generated || state.Entries[0].ContentSHA256 == "" {
		t.Fatalf("mount state should record a generated entry with a content hash: %#v", state.Entries)
	}

	again, err := Apply(project, paths, []SourceEntry{entry}, staticResolver{action: ConflictSkip})
	if err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	if again.AlreadyLinked != 1 || again.Linked != 0 {
		t.Fatalf("re-applying an identical generated file should be a no-op: %#v", again)
	}

	unmountResult, err := Unmount(project, Selection{})
	if err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
	if unmountResult.Removed != 1 || len(unmountResult.KeptModified) != 0 {
		t.Fatalf("Unmount() result = %#v", unmountResult)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("generated file should be removed on unmount")
	}
}

func TestUnmountPreservesEditedGeneratedFile(t *testing.T) {
	project, paths, entry := generatedBridgeFixture(t)
	target := filepath.Join(project, "CLAUDE.md")

	if _, err := Apply(project, paths, []SourceEntry{entry}, staticResolver{action: ConflictOverwrite}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := os.WriteFile(target, []byte("@AGENTS.md\n\n## my additions\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := Unmount(project, Selection{})
	if err != nil {
		t.Fatalf("Unmount() error = %v", err)
	}
	if len(result.KeptModified) != 1 {
		t.Fatalf("expected the edited file to be kept, got %#v", result)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("edited generated file should leave the project root")
	}
	kept, err := os.ReadFile(result.KeptModified[0])
	if err != nil {
		t.Fatalf("kept file unreadable: %v", err)
	}
	if string(kept) != "@AGENTS.md\n\n## my additions\n" {
		t.Fatalf("kept content = %q", kept)
	}
	if !strings.Contains(filepath.ToSlash(result.KeptModified[0]), ".csaw-stash/modified/CLAUDE.md") {
		t.Fatalf("kept path = %q, want it under .csaw-stash/modified/", result.KeptModified[0])
	}
}

func TestRepairRewritesMissingGeneratedFile(t *testing.T) {
	project, paths, entry := generatedBridgeFixture(t)
	target := filepath.Join(project, "CLAUDE.md")

	if _, err := Apply(project, paths, []SourceEntry{entry}, staticResolver{action: ConflictOverwrite}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	result, statuses, err := Repair(project)
	if err != nil {
		t.Fatalf("Repair() error = %v", err)
	}
	if result.Linked != 1 {
		t.Fatalf("Repair() result = %#v, want 1 rewritten", result)
	}
	for _, status := range statuses {
		if !status.Healthy {
			t.Fatalf("status after repair = %#v", status)
		}
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != entry.Content {
		t.Fatalf("rewritten content = %q (err %v), want %q", content, err, entry.Content)
	}
}

func TestEntriesFromMountedStateCarriesGeneratedContent(t *testing.T) {
	entries := EntriesFromMountedState(workspace.MountState{Entries: []workspace.MountedStateEntry{{
		RelativePath: "CLAUDE.md", SourceName: "team", SourcePath: "/team/AGENTS.md", Generated: true, Content: "@AGENTS.md\n",
	}}})
	if len(entries) != 1 || !entries[0].Generated || entries[0].Content != "@AGENTS.md\n" {
		t.Fatalf("restore entries lost generated content: %#v", entries)
	}
}
