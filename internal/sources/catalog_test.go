package sources

import (
	"context"
	"errors"
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

func TestPullDirtySourceError(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	sourceDir := filepath.Join(paths.Sources, "team")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	git := &recordingGit{
		outputs: map[string]string{
			joinArgs([]string{"status", "--porcelain"}): " M AGENTS.md",
		},
	}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{Name: "team", Kind: KindRemote, URL: "git@example.com:org/repo.git"}); err != nil {
		t.Fatal(err)
	}

	err := manager.Pull(context.Background(), "team", false)
	var dirtyErr *DirtySourceError
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("Pull() error = %v, want DirtySourceError", err)
	}
	if dirtyErr.Source != "team" {
		t.Fatalf("DirtySourceError.Source = %q, want %q", dirtyErr.Source, "team")
	}
}

func TestPullDirtyWithStash(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	sourceDir := filepath.Join(paths.Sources, "team")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	git := &recordingGit{
		outputs: map[string]string{
			joinArgs([]string{"status", "--porcelain"}): " M AGENTS.md",
		},
	}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{Name: "team", Kind: KindRemote, URL: "git@example.com:org/repo.git"}); err != nil {
		t.Fatal(err)
	}

	err := manager.Pull(context.Background(), "team", true)
	if err != nil {
		t.Fatalf("Pull(stash=true) error = %v", err)
	}

	// Should have called: status, stash, pull, stash pop
	var commands []string
	for _, call := range git.calls {
		if len(call) > 1 {
			commands = append(commands, call[1])
		}
	}

	hasStash := false
	hasFetch := false
	for _, cmd := range commands {
		if cmd == "stash" {
			hasStash = true
		}
		if cmd == "fetch" {
			hasFetch = true
		}
	}
	if !hasStash {
		t.Error("expected stash command")
	}
	if !hasFetch {
		t.Error("expected fetch command")
	}
}

func TestPullCleanSource(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	sourceDir := filepath.Join(paths.Sources, "team")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	git := &recordingGit{
		outputs: map[string]string{
			joinArgs([]string{"status", "--porcelain"}): "",
		},
	}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{Name: "team", Kind: KindRemote, URL: "git@example.com:org/repo.git"}); err != nil {
		t.Fatal(err)
	}

	err := manager.Pull(context.Background(), "team", false)
	if err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
}

func TestPullDivergedSource(t *testing.T) {
	root := t.TempDir()
	paths := runtime.BuildPaths(filepath.Join(root, ".csaw"))

	sourceDir := filepath.Join(paths.Sources, "team")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	git := &recordingGit{
		outputs: map[string]string{
			joinArgs([]string{"status", "--porcelain"}):             "",
			joinArgs([]string{"rev-list", "--count", "HEAD..@{u}"}): "3",
			joinArgs([]string{"rev-list", "--count", "@{u}..HEAD"}): "2",
		},
	}
	manager := Manager{Paths: paths, Git: git}
	if err := manager.Add(Source{Name: "team", Kind: KindRemote, URL: "git@example.com:org/repo.git"}); err != nil {
		t.Fatal(err)
	}

	err := manager.Pull(context.Background(), "team", false)
	var divErr *DivergedSourceError
	if !errors.As(err, &divErr) {
		t.Fatalf("Pull() error = %v, want DivergedSourceError", err)
	}
	if divErr.Ahead != 2 || divErr.Behind != 3 {
		t.Fatalf("DivergedSourceError ahead=%d behind=%d, want 2/3", divErr.Ahead, divErr.Behind)
	}
}

func TestValidateConfigDuplicates(t *testing.T) {
	cfg := Config{
		Sources: []Source{
			{Name: "team", Kind: KindRemote},
			{Name: "team", Kind: KindLocal},
		},
	}
	warnings := ValidateConfig(cfg)
	if len(warnings) == 0 {
		t.Fatal("expected warning for duplicate source name")
	}
}

func TestValidateConfigBadForkTarget(t *testing.T) {
	cfg := Config{
		Sources:           []Source{{Name: "team", Kind: KindRemote}},
		DefaultForkTarget: "nonexistent",
	}
	warnings := ValidateConfig(cfg)
	if len(warnings) == 0 {
		t.Fatal("expected warning for invalid default_fork_target")
	}
}

func TestValidateConfigClean(t *testing.T) {
	cfg := Config{
		Sources:           []Source{{Name: "team", Kind: KindRemote}},
		DefaultForkTarget: "team",
	}
	warnings := ValidateConfig(cfg)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", warnings)
	}
}
