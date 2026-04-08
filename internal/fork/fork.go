package fork

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/csaw-ai/csaw/internal/sources"
)

type Result struct {
	FromSource   string
	FromPath     string
	IntoSource   string
	IntoPath     string
	RelativePath string
}

// Fork copies a file from one source into another.
// qualifiedPath is "source/relative/path" (e.g., "team/agents/base.md").
// into is the target source name.
func Fork(qualifiedPath string, into string, catalog []sources.CatalogSource) (Result, error) {
	parts := strings.SplitN(qualifiedPath, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Result{}, fmt.Errorf("invalid qualified path %q: expected source/path", qualifiedPath)
	}
	sourceName, relativePath := parts[0], parts[1]

	if sourceName == into {
		return Result{}, fmt.Errorf("source and target are the same: %s", sourceName)
	}

	var fromRoot, intoRoot string
	for _, entry := range catalog {
		if entry.Name == sourceName {
			fromRoot = entry.Root
		}
		if entry.Name == into {
			intoRoot = entry.Root
		}
	}

	if fromRoot == "" {
		return Result{}, fmt.Errorf("source %q not found", sourceName)
	}
	if intoRoot == "" {
		return Result{}, fmt.Errorf("target source %q not found", into)
	}

	fromPath := filepath.Join(fromRoot, filepath.FromSlash(relativePath))
	if _, err := os.Stat(fromPath); err != nil {
		return Result{}, fmt.Errorf("file not found: %s", fromPath)
	}

	intoPath := filepath.Join(intoRoot, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(intoPath), 0o755); err != nil {
		return Result{}, err
	}

	content, err := os.ReadFile(fromPath)
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(intoPath, content, 0o644); err != nil {
		return Result{}, err
	}

	return Result{
		FromSource:   sourceName,
		FromPath:     fromPath,
		IntoSource:   into,
		IntoPath:     intoPath,
		RelativePath: relativePath,
	}, nil
}
