package fork

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/sources"
)

func TestFork(t *testing.T) {
	root := t.TempDir()
	teamDir := filepath.Join(root, "team")
	personalDir := filepath.Join(root, "personal")

	if err := os.MkdirAll(filepath.Join(teamDir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(personalDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sourceFile := filepath.Join(teamDir, "agents", "base.md")
	if err := os.WriteFile(sourceFile, []byte("team rules"), 0o644); err != nil {
		t.Fatal(err)
	}

	catalog := []sources.CatalogSource{
		{Name: "team", Root: teamDir},
		{Name: "personal", Root: personalDir},
	}

	result, err := Fork("team/agents/base.md", "personal", catalog)
	if err != nil {
		t.Fatalf("Fork() error = %v", err)
	}

	if result.FromSource != "team" || result.IntoSource != "personal" {
		t.Fatalf("unexpected result: %+v", result)
	}

	content, err := os.ReadFile(filepath.Join(personalDir, "agents", "base.md"))
	if err != nil {
		t.Fatalf("forked file not found: %v", err)
	}
	if string(content) != "team rules" {
		t.Fatalf("content = %q, want %q", string(content), "team rules")
	}
}

func TestForkMissingSource(t *testing.T) {
	catalog := []sources.CatalogSource{
		{Name: "personal", Root: t.TempDir()},
	}

	_, err := Fork("nonexistent/agents/base.md", "personal", catalog)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestForkMissingFile(t *testing.T) {
	catalog := []sources.CatalogSource{
		{Name: "team", Root: t.TempDir()},
		{Name: "personal", Root: t.TempDir()},
	}

	_, err := Fork("team/agents/missing.md", "personal", catalog)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestForkSameSource(t *testing.T) {
	catalog := []sources.CatalogSource{
		{Name: "team", Root: t.TempDir()},
	}

	_, err := Fork("team/agents/base.md", "team", catalog)
	if err == nil {
		t.Fatal("expected error when source == target")
	}
}
