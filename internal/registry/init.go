package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NicholasCullenCooper/csaw/internal/git"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
)

type InitResult struct {
	Path string
	Name string
}

var starterProfile = `default:
  description: Default instructions, rules, agents, and skills
  include:
    - AGENTS.md
    - rules/**
    - skills/**
    - agents/**
`

var starterIgnore = `# Patterns listed here are excluded from mounting by default.
# Use --include-ignored to mount them anyway.
#
# Note: Files under any "experimental/" path segment are hidden by csaw's
# built-in convention (no need to list them here). Use --include-experimental
# to mount those.
#
# Example patterns (uncomment to use):
# drafts/**
# archived/**
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

// Init scaffolds a new csaw registry directory. If preset is empty, writes
// the default (solo-style) starter set. If preset names a registered preset
// (see presets.go), writes that preset's file set instead. Returns an error
// if a non-empty preset name doesn't match any built-in.
//
// Existing files in the target directory are never overwritten — Init only
// creates files that don't already exist. Safe to run on a populated dir.
func Init(ctx context.Context, g git.Git, dir string, name string, preset string) (InitResult, error) {
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

	files := defaultStarterFiles()
	if preset != "" {
		p, ok := GetPreset(preset)
		if !ok {
			return InitResult{}, fmt.Errorf("unknown preset %q (run `csaw init --list-presets` to see options)", preset)
		}
		files = p.Files
	}

	for path, content := range files {
		fullPath := filepath.Join(absDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return InitResult{}, err
		}
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
				return InitResult{}, err
			}
		}
	}

	// Always ensure the historical convention dirs exist even if the preset
	// didn't seed files in them — keeps `csaw inspect` and `--kind` filters
	// behaving as users expect on a freshly-created source.
	for _, sub := range []string{"rules", "agents", "skills"} {
		if err := os.MkdirAll(filepath.Join(absDir, sub), 0o755); err != nil {
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

// defaultStarterFiles is the file set used when `csaw init` is run with no
// `--preset` flag. Preserves the historical scaffold (csaw.yml, .csawignore,
// AGENTS.md, code-review + commit-message skills).
func defaultStarterFiles() map[string]string {
	return map[string]string{
		"csaw.yml":                       starterProfile,
		".csawignore":                    starterIgnore,
		"AGENTS.md":                      starterAgents,
		"skills/code-review/SKILL.md":    starterSkillCodeReview,
		"skills/commit-message/SKILL.md": starterSkillCommitMsg,
	}
}

// AdoptResult extends InitResult with the list of files adopted from a project.
type AdoptResult struct {
	InitResult
	AdoptedFiles []string // registry-relative paths of adopted files
}

// InitWithAdopt scaffolds a registry and adopts AI config files from a project.
// It scans the project for skills, agent instructions, MCP configs, and root
// instruction files, copies them into the registry, and generates a profile.
func InitWithAdopt(ctx context.Context, g git.Git, dir string, name string, projectRoot string) (AdoptResult, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return AdoptResult{}, err
	}

	adoptable := mount.ScanAdoptableFiles(projectRoot)
	preExisting := make(map[string]bool, len(adoptable))
	for _, file := range adoptable {
		destPath := filepath.Join(absDir, filepath.FromSlash(file.RegistryPath))
		if _, err := os.Stat(destPath); err == nil {
			preExisting[file.RegistryPath] = true
		} else if err != nil && !os.IsNotExist(err) {
			return AdoptResult{}, err
		}
	}

	initResult, err := Init(ctx, g, dir, name, "")
	if err != nil {
		return AdoptResult{}, err
	}

	if len(adoptable) == 0 {
		return AdoptResult{InitResult: initResult}, nil
	}

	var adopted []string
	for _, file := range adoptable {
		destPath := filepath.Join(initResult.Path, filepath.FromSlash(file.RegistryPath))

		// Preserve files that existed before init. Starter files created by Init
		// are overwritten by project-owned context during adoption.
		if preExisting[file.RegistryPath] {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return AdoptResult{}, err
		}

		srcPath := filepath.Join(projectRoot, filepath.FromSlash(file.ProjectPath))
		content, err := os.ReadFile(srcPath)
		if err != nil {
			return AdoptResult{}, err
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return AdoptResult{}, err
		}

		adopted = append(adopted, file.RegistryPath)
	}

	// Generate a profile covering all adopted files
	if len(adopted) > 0 {
		profileContent := generateAdoptProfile(adopted)
		profilePath := filepath.Join(initResult.Path, "csaw.yml")
		if err := os.WriteFile(profilePath, []byte(profileContent), 0o644); err != nil {
			return AdoptResult{}, err
		}
	}

	return AdoptResult{InitResult: initResult, AdoptedFiles: adopted}, nil
}

func generateAdoptProfile(files []string) string {
	// Collect top-level directory patterns
	patterns := make(map[string]bool)
	for _, f := range files {
		parts := strings.SplitN(f, "/", 2)
		if len(parts) == 1 {
			// Root file like AGENTS.md
			patterns[f] = true
		} else {
			// Directory like skills/foo/SKILL.md → skills/**
			patterns[parts[0]+"/**"] = true
		}
	}

	var b strings.Builder
	b.WriteString("default:\n")
	b.WriteString("  description: Adopted from project\n")
	b.WriteString("  include:\n")

	sorted := make([]string, 0, len(patterns))
	for p := range patterns {
		sorted = append(sorted, p)
	}
	// Sort for deterministic output
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for _, p := range sorted {
		fmt.Fprintf(&b, "    - %s\n", p)
	}

	return b.String()
}
