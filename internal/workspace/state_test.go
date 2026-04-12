package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

func TestStashAndRestore(t *testing.T) {
	project := t.TempDir()
	target := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(target, []byte("local"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := FileStateStore{}
	if err := StashFile(store, project, "AGENTS.md", "/tmp/source"); err != nil {
		t.Fatalf("StashFile() error = %v", err)
	}

	if err := os.WriteFile(target, []byte("changed"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	restored, err := RestoreFile(store, project, "AGENTS.md")
	if err != nil {
		t.Fatalf("RestoreFile() error = %v", err)
	}
	if !restored {
		t.Fatal("RestoreFile() restored = false, want true")
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "local" {
		t.Fatalf("restored content = %q, want %q", string(content), "local")
	}
}

func TestAddExclusion(t *testing.T) {
	project := t.TempDir()
	gitInfo := filepath.Join(project, ".git", "info")
	if err := os.MkdirAll(gitInfo, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	added, err := AddExclusion(project, "AGENTS.md")
	if err != nil {
		t.Fatalf("AddExclusion() error = %v", err)
	}
	if !added {
		t.Fatal("AddExclusion() added = false, want true")
	}

	lines, err := ReadExclude(project)
	if err != nil {
		t.Fatalf("ReadExclude() error = %v", err)
	}
	if len(lines) < 2 || lines[len(lines)-2] != runtime.ManagedComment || lines[len(lines)-1] != "AGENTS.md" {
		t.Fatalf("exclude contents = %v", lines)
	}
}
