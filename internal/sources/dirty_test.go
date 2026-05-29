package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

func TestParseDirtyFiles(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []DirtyFile
	}{
		{"empty", "", nil},
		{"only whitespace", "   \n", nil},
		{
			"single modified",
			" M rules/security.md",
			[]DirtyFile{{Path: "rules/security.md", Status: "M"}},
		},
		{
			"multiple statuses",
			"?? new-file.md\n M rules/x.md\nA  agents/y.md",
			[]DirtyFile{
				{Path: "new-file.md", Status: "??"},
				{Path: "rules/x.md", Status: "M"},
				{Path: "agents/y.md", Status: "A"},
			},
		},
		{
			"both-side modified (MM)",
			"MM rules/contested.md",
			[]DirtyFile{{Path: "rules/contested.md", Status: "MM"}},
		},
		{
			"too-short lines skipped",
			"x\n M rules/ok.md\n",
			[]DirtyFile{{Path: "rules/ok.md", Status: "M"}},
		},
	}

	for _, tc := range cases {
		got := parseDirtyFiles(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %d files, want %d (%v vs %v)", tc.name, len(got), len(tc.want), got, tc.want)
			continue
		}
		for i, g := range got {
			if g != tc.want[i] {
				t.Errorf("%s [%d]: got %+v, want %+v", tc.name, i, g, tc.want[i])
			}
		}
	}
}

// TestDirtyFilesNoCheckoutReturnsNil verifies that asking about a remote
// source that hasn't been cloned yet returns nil (no error) — the caller
// shouldn't have to know whether the source is pulled.
func TestDirtyFilesNoCheckoutReturnsNil(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	git := &recordingGit{}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{
		Name: "team",
		Kind: KindRemote,
		URL:  "git@example.com:org/repo.git",
	}); err != nil {
		t.Fatal(err)
	}

	files, err := manager.DirtyFiles(context.Background(), "team")
	if err != nil {
		t.Fatalf("DirtyFiles() error = %v, want nil", err)
	}
	if files != nil {
		t.Errorf("DirtyFiles() = %v, want nil for un-cloned source", files)
	}
}

// TestDirtyFilesNonGitDirReturnsNil verifies that a checkout dir that isn't
// a git repo returns nil instead of erroring. Defends against local sources
// that point at non-git directories.
func TestDirtyFilesNonGitDirReturnsNil(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	// Create source dir without a .git subdirectory
	sourceDir := filepath.Join(paths.Sources, "team")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	git := &recordingGit{}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{Name: "team", Kind: KindRemote, URL: "git@x.com:o/r.git"}); err != nil {
		t.Fatal(err)
	}

	files, err := manager.DirtyFiles(context.Background(), "team")
	if err != nil {
		t.Fatalf("DirtyFiles() error = %v", err)
	}
	if files != nil {
		t.Errorf("DirtyFiles() = %v, want nil for non-git checkout", files)
	}
}

// TestDirtyFilesReportsFromGitStatus integrates parseDirtyFiles with the
// git command, verifying the output of `git status --porcelain` is correctly
// parsed and returned per file.
func TestDirtyFilesReportsFromGitStatus(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	sourceDir := filepath.Join(paths.Sources, "team")
	if err := os.MkdirAll(filepath.Join(sourceDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	git := &recordingGit{
		outputs: map[string]string{
			joinArgs([]string{"status", "--porcelain"}): " M rules/security.md\n?? new-agent.md",
		},
	}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{Name: "team", Kind: KindRemote, URL: "git@x.com:o/r.git"}); err != nil {
		t.Fatal(err)
	}

	files, err := manager.DirtyFiles(context.Background(), "team")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
	if files[0].Path != "rules/security.md" || files[0].Status != "M" {
		t.Errorf("files[0] = %+v", files[0])
	}
	if files[1].Path != "new-agent.md" || files[1].Status != "??" {
		t.Errorf("files[1] = %+v", files[1])
	}
}
