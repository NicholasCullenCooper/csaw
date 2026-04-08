package registry

import (
	"context"
	"os"
	"path/filepath"

	"github.com/csaw-ai/csaw/internal/git"
)

type InitResult struct {
	Path string
	Name string
}

var starterProfile = `# Profiles define named sets of files to mount.
# See: https://github.com/csaw-ai/csaw
#
# default:
#   include:
#     - agents/**
#     - skills/**
`

var starterIgnore = `# Patterns listed here are excluded from mounting by default.
# Use --include-ignored to override.
`

func Init(ctx context.Context, g git.Git, dir string, name string) (InitResult, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return InitResult{}, err
	}

	if name == "" {
		name = filepath.Base(absDir)
	}

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return InitResult{}, err
	}

	for _, sub := range []string{"agents", "skills"} {
		if err := os.MkdirAll(filepath.Join(absDir, sub), 0o755); err != nil {
			return InitResult{}, err
		}
	}

	profilesPath := filepath.Join(absDir, "csaw.yml")
	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		if err := os.WriteFile(profilesPath, []byte(starterProfile), 0o644); err != nil {
			return InitResult{}, err
		}
	}

	ignorePath := filepath.Join(absDir, ".csawignore")
	if _, err := os.Stat(ignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(ignorePath, []byte(starterIgnore), 0o644); err != nil {
			return InitResult{}, err
		}
	}

	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if _, err := g.Run(ctx, absDir, "init"); err != nil {
			return InitResult{}, err
		}
	}

	return InitResult{Path: absDir, Name: name}, nil
}
