package mount

import (
	"os"
	"path/filepath"
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
