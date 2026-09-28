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

func TestInspectMountStateDetectsProtectedContentDrift(t *testing.T) {
	if canSymlink, reason := canCreateSymlink(); !canSymlink {
		t.Skip(reason)
	}

	root := t.TempDir()
	project := filepath.Join(root, "project")
	sourceDir := filepath.Join(root, "source")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}

	source := filepath.Join(sourceDir, "AGENTS.md")
	target := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(source, []byte("approved"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	approvedHash, err := workspace.FileSHA256(source)
	if err != nil {
		t.Fatalf("FileSHA256() error = %v", err)
	}
	if err := os.Symlink(source, target); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	if err := os.WriteFile(source, []byte("changed"), 0o644); err != nil {
		t.Fatalf("WriteFile(changed source) error = %v", err)
	}

	statuses := InspectMountState(project, workspace.MountState{
		Entries: []workspace.MountedStateEntry{{
			RelativePath: "AGENTS.md",
			SourceName:   "team",
			SourcePath:   source,
			Protected:    true,
			SourceSHA256: approvedHash,
		}},
	}, linkmode.Symlink)
	if got, want := len(statuses), 1; got != want {
		t.Fatalf("len(statuses) = %d, want %d", got, want)
	}
	if statuses[0].Healthy {
		t.Fatalf("protected content drift should be unhealthy: %+v", statuses[0])
	}
	if statuses[0].Issue != IssueProtectedContentDrift {
		t.Fatalf("Issue = %q, want %q", statuses[0].Issue, IssueProtectedContentDrift)
	}
	if statuses[0].ExpectedHash != approvedHash {
		t.Fatalf("ExpectedHash = %q, want %q", statuses[0].ExpectedHash, approvedHash)
	}
	if statuses[0].ActualHash == "" || statuses[0].ActualHash == approvedHash {
		t.Fatalf("ActualHash = %q, want changed hash", statuses[0].ActualHash)
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

func TestInspectMountStateClassifiesGeneratedFiles(t *testing.T) {
	project := t.TempDir()
	target := filepath.Join(project, "CLAUDE.md")
	content := []byte("@AGENTS.md\n")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	entry := workspace.MountedStateEntry{
		RelativePath:  "CLAUDE.md",
		SourceName:    "team",
		SourcePath:    filepath.Join(project, "does-not-matter", "AGENTS.md"),
		Generated:     true,
		Content:       string(content),
		ContentSHA256: workspace.ContentSHA256(content),
	}
	state := workspace.MountState{Entries: []workspace.MountedStateEntry{entry}}

	statuses := InspectMountState(project, state, linkmode.Symlink)
	if len(statuses) != 1 || !statuses[0].Healthy || !statuses[0].Generated {
		t.Fatalf("intact generated file should be healthy: %#v", statuses)
	}

	if err := os.WriteFile(target, []byte("@AGENTS.md\nextra\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	statuses = InspectMountState(project, state, linkmode.Symlink)
	if statuses[0].Healthy || statuses[0].Issue != IssueGeneratedModified {
		t.Fatalf("edited generated file should report %s: %#v", IssueGeneratedModified, statuses[0])
	}

	if err := os.Remove(target); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	statuses = InspectMountState(project, state, linkmode.Symlink)
	if statuses[0].Healthy || statuses[0].Issue != IssueMissingLink {
		t.Fatalf("missing generated file should report %s: %#v", IssueMissingLink, statuses[0])
	}
}
