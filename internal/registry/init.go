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

var starterProfile = `default:
  description: Mount everything
  include:
    - AGENTS.md
    - skills/**
`

var starterIgnore = `# Patterns listed here are excluded from mounting by default.
# Use --include-ignored to override.
`

var starterAgents = `# Agent Instructions

## Code Style
- Write clear, readable code with meaningful names
- Keep functions focused and small
- Add comments only where the logic isn't self-evident

## Workflow
- Run tests before committing
- Write descriptive commit messages
- Keep PRs focused on a single concern

## Preferences
- Prefer simple solutions over clever ones
- Fix the root cause, not the symptom
- Leave code cleaner than you found it
`

var starterSkillCodeReview = `---
name: code-review
description: Thorough, constructive code review
---

When reviewing code:

1. **Correctness first** — Does it do what it claims? Are there edge cases?
2. **Readability** — Can someone unfamiliar with the code understand it?
3. **Simplicity** — Is there a simpler way to achieve the same result?
4. **Tests** — Are the important paths tested? Are tests clear and maintainable?
5. **Security** — Any injection risks, auth issues, or data exposure?

Be specific in feedback. Instead of "this is confusing", say what's confusing and suggest an alternative. Acknowledge good decisions, not just problems.
`

var starterSkillCommitMsg = `---
name: commit-message
description: Write clear, conventional commit messages
---

When writing commit messages:

- Use the imperative mood ("Add feature" not "Added feature")
- Keep the subject line under 72 characters
- Separate subject from body with a blank line
- Use the body to explain what and why, not how
- Reference issues and PRs where relevant
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

	for _, sub := range []string{"skills/code-review", "skills/commit-message"} {
		if err := os.MkdirAll(filepath.Join(absDir, sub), 0o755); err != nil {
			return InitResult{}, err
		}
	}

	// Write starter files only if they don't exist
	starters := []struct {
		path    string
		content string
	}{
		{"csaw.yml", starterProfile},
		{".csawignore", starterIgnore},
		{"AGENTS.md", starterAgents},
		{"skills/code-review/SKILL.md", starterSkillCodeReview},
		{"skills/commit-message/SKILL.md", starterSkillCommitMsg},
	}

	for _, s := range starters {
		fullPath := filepath.Join(absDir, s.path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			if err := os.WriteFile(fullPath, []byte(s.content), 0o644); err != nil {
				return InitResult{}, err
			}
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
