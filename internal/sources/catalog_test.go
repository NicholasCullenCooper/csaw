package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/csaw-ai/csaw/internal/runtime"
)

type recordingGit struct {
	calls   [][]string
	outputs map[string]string
}

func (g *recordingGit) Run(ctx context.Context, cwd string, args ...string) (string, error) {
	_ = ctx
	call := append([]string{cwd}, args...)
	g.calls = append(g.calls, call)
	if g.outputs == nil {
		return "", nil
	}
	return g.outputs[joinArgs(args)], nil
}

func joinArgs(values []string) string {
	return filepath.ToSlash(filepath.Join(values...))
}

func TestCatalogFromConfig(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))
	manager := Manager{Paths: paths}

	catalog, err := manager.Catalog()
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if len(catalog) != 0 {
		t.Fatalf("Catalog() = %#v, want empty catalog", catalog)
	}
}

func TestCatalogPreservesPriority(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	sourceDir := filepath.Join(root, "my-source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manager := Manager{Paths: paths}
	if err := manager.Add(Source{Name: "high", Kind: KindLocal, Path: sourceDir, Priority: 10}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Add(Source{Name: "low", Kind: KindLocal, Path: sourceDir, Priority: 0}); err != nil {
		t.Fatal(err)
	}

	catalog, err := manager.Catalog()
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if len(catalog) != 2 {
		t.Fatalf("Catalog() len = %d, want 2", len(catalog))
	}

	// Catalog is sorted alphabetically
	for _, entry := range catalog {
		if entry.Name == "high" && entry.Priority != 10 {
			t.Fatalf("high priority = %d, want 10", entry.Priority)
		}
		if entry.Name == "low" && entry.Priority != 0 {
			t.Fatalf("low priority = %d, want 0", entry.Priority)
		}
	}
}

func TestPush(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	sourceDir := filepath.Join(root, "my-source")
	sourceGit := filepath.Join(sourceDir, ".git")
	if err := os.MkdirAll(sourceGit, 0o755); err != nil {
		t.Fatal(err)
	}

	git := &recordingGit{
		outputs: map[string]string{
			joinArgs([]string{"status", "--porcelain"}): " M AGENTS.md",
		},
	}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{Name: "team", Kind: KindLocal, Path: sourceDir}); err != nil {
		t.Fatal(err)
	}

	if err := manager.Push(context.Background(), "team", "test commit"); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if got, want := len(git.calls), 4; got != want {
		t.Fatalf("len(calls) = %d, want %d", got, want)
	}
}

func TestPushNothingToPush(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	sourceDir := filepath.Join(root, "my-source")
	sourceGit := filepath.Join(sourceDir, ".git")
	if err := os.MkdirAll(sourceGit, 0o755); err != nil {
		t.Fatal(err)
	}

	git := &recordingGit{
		outputs: map[string]string{
			joinArgs([]string{"status", "--porcelain"}): "",
		},
	}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{Name: "team", Kind: KindLocal, Path: sourceDir}); err != nil {
		t.Fatal(err)
	}

	err := manager.Push(context.Background(), "team", "test commit")
	if err != ErrNothingToPush {
		t.Fatalf("Push() error = %v, want ErrNothingToPush", err)
	}
}
