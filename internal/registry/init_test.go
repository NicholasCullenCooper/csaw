package registry

import (
	"context"
	"os"
	"path/filepath"
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

	for _, sub := range []string{"agents", "skills"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", sub)
		}
	}

	for _, file := range []string{"csaw.yml", ".csawignore"} {
		if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
			t.Fatalf("Stat(%s) error = %v", file, err)
		}
	}

	if len(g.calls) != 1 || g.calls[0][1] != "init" {
		t.Fatalf("git calls = %v, want [init]", g.calls)
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
