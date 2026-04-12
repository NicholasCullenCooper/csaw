package drift

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/linkmode"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

func TestInspectMountStateClassifiesDrift(t *testing.T) {
	if canSymlink, reason := canCreateSymlink(); !canSymlink {
		t.Skip(reason)
	}

	root := t.TempDir()
	project := filepath.Join(root, "project")
	sourceDir := filepath.Join(root, "source")
	wrongDir := filepath.Join(root, "wrong")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(wrongDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	expected := filepath.Join(sourceDir, "AGENTS.md")
	wrong := filepath.Join(wrongDir, "AGENTS.md")
	target := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(expected, []byte("expected"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(wrong, []byte("wrong"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink(wrong, target); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	statuses := InspectMountState(project, workspace.MountState{
		Entries: []workspace.MountedStateEntry{{
			RelativePath: "AGENTS.md",
			SourceName:   "personal",
			SourcePath:   expected,
		}},
	}, linkmode.Symlink)
	if got, want := len(statuses), 1; got != want {
		t.Fatalf("len(statuses) = %d, want %d", got, want)
	}
	if statuses[0].Issue != IssueDriftedLink {
		t.Fatalf("Issue = %q, want %q", statuses[0].Issue, IssueDriftedLink)
	}
}

func canCreateSymlink() (bool, string) {
	root, err := os.MkdirTemp("", "csaw-drift-*")
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
