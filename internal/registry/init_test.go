package registry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingGit struct {
	calls [][]string
}

func (g *recordingGit) Run(ctx context.Context, cwd string, args ...string) (string, error) {
	_ = ctx
	call := append([]string{cwd}, args...)
	g.calls = append(g.calls, call)
	return "", nil
}

func TestInit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-config")
	g := &recordingGit{}

	result, err := Init(context.Background(), g, dir, "")
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if result.Name != "my-config" {
		t.Fatalf("Name = %q, want %q", result.Name, "my-config")
	}

	for _, sub := range []string{"skills/code-review", "skills/commit-message"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", sub)
		}
	}

	for _, file := range []string{"csaw.yml", ".csawignore", "AGENTS.md", "skills/code-review/SKILL.md", "skills/commit-message/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
			t.Fatalf("Stat(%s) error = %v", file, err)
		}
	}

	// Verify csaw.yml has a real default profile
	content, err := os.ReadFile(filepath.Join(dir, "csaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "default:") {
		t.Fatal("csaw.yml should contain a default profile")
	}

	if len(g.calls) != 1 || g.calls[0][1] != "init" {
		t.Fatalf("git calls = %v, want [init]", g.calls)
	}
}

func TestInitWithAdopt(t *testing.T) {
	// Set up a fake project with AI config files
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, ".claude", "skills", "testing"), 0o755)
	os.MkdirAll(filepath.Join(project, ".claude", "rules"), 0o755)
	os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("team rules"), 0o644)
	os.WriteFile(filepath.Join(project, ".claude", "skills", "testing", "SKILL.md"), []byte("test skill"), 0o644)
	os.WriteFile(filepath.Join(project, ".claude", "rules", "go.md"), []byte("go rules"), 0o644)

	registryDir := filepath.Join(t.TempDir(), "my-registry")
	g := &recordingGit{}

	result, err := InitWithAdopt(context.Background(), g, registryDir, "", project)
	if err != nil {
		t.Fatalf("InitWithAdopt() error = %v", err)
	}

	if len(result.AdoptedFiles) != 2 {
		// AGENTS.md exists from starter (skipped), but skills/testing and agents/go.md should be adopted
		t.Fatalf("AdoptedFiles = %v, want 2 files", result.AdoptedFiles)
	}

	// Verify skill was copied
	content, err := os.ReadFile(filepath.Join(registryDir, "skills", "testing", "SKILL.md"))
	if err != nil {
		t.Fatalf("adopted skill not found: %v", err)
	}
	if string(content) != "test skill" {
		t.Fatalf("skill content = %q, want %q", string(content), "test skill")
	}

	// Verify rule was copied (now goes to rules/, not agents/)
	content, err = os.ReadFile(filepath.Join(registryDir, "rules", "go.md"))
	if err != nil {
		t.Fatalf("adopted rule not found: %v", err)
	}
	if string(content) != "go rules" {
		t.Fatalf("rule content = %q, want %q", string(content), "go rules")
	}

	// Verify csaw.yml was updated with adopted patterns
	profileContent, err := os.ReadFile(filepath.Join(registryDir, "csaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(profileContent), "skills/**") {
		t.Fatalf("csaw.yml should include skills/**, got:\n%s", string(profileContent))
	}
}

func TestInitExistingDir(t *testing.T) {
	dir := t.TempDir()
	// Pre-create csaw.yml to verify it's not overwritten
	existing := "existing content"
	if err := os.WriteFile(filepath.Join(dir, "csaw.yml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-create .git to verify git init is skipped
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	g := &recordingGit{}
	_, err := Init(context.Background(), g, dir, "custom-name")
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// csaw.yml should not be overwritten
	content, err := os.ReadFile(filepath.Join(dir, "csaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != existing {
		t.Fatalf("csaw.yml was overwritten: %q", string(content))
	}

	// git init should not be called (already a git repo)
	if len(g.calls) != 0 {
		t.Fatalf("git calls = %v, want none", g.calls)
	}
}
