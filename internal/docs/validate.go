package docs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

var markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

var requiredExecPlanHeadings = []string{
	"## Summary",
	"## Success Criteria",
	"## Workstreams",
	"## Risks",
	"## Validation",
}

var forbiddenDocPatterns = []*regexp.Regexp{
	regexp.MustCompile(`/Users/`),
	regexp.MustCompile(`C:\\\\Users\\\\`),
}

var forbiddenSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
}

var forbiddenPublicPaths []string

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func ValidateAgentsLinks(root string) error {
	content, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return err
	}

	matches := markdownLinkPattern.FindAllStringSubmatch(string(content), -1)
	for _, match := range matches {
		target := match[1]
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
			continue
		}

		target = strings.SplitN(target, "#", 2)[0]
		if target == "" {
			continue
		}

		fullPath := filepath.Join(root, filepath.FromSlash(target))
		if _, err := os.Stat(fullPath); err != nil {
			return fmt.Errorf("AGENTS.md link target missing: %s", target)
		}
	}

	return nil
}

func ValidateActiveExecPlans(root string) error {
	dir := filepath.Join(root, "docs", "exec-plans", "active")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}

		text := string(content)
		for _, heading := range requiredExecPlanHeadings {
			if !strings.Contains(text, heading) {
				return fmt.Errorf("%s is missing required heading %q", entry.Name(), heading)
			}
		}
	}

	return nil
}

func ValidateSkills(root string) error {
	skillFiles, err := filepath.Glob(filepath.Join(root, "skills", "*", "SKILL.md"))
	if err != nil {
		return err
	}
	if len(skillFiles) == 0 {
		return errors.New("no skill files found")
	}

	for _, skillFile := range skillFiles {
		content, err := os.ReadFile(skillFile)
		if err != nil {
			return err
		}

		frontmatter, err := extractFrontmatter(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", skillFile, err)
		}

		var metadata skillFrontmatter
		if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
			return fmt.Errorf("%s: %w", skillFile, err)
		}
		if metadata.Name == "" || metadata.Description == "" {
			return fmt.Errorf("%s: missing required name or description", skillFile)
		}
	}

	return nil
}

func ValidatePublicRepoContent(root string) error {
	files, err := trackedFiles(root)
	if err != nil {
		return err
	}

	for _, file := range files {
		for _, forbidden := range forbiddenPublicPaths {
			if filepath.ToSlash(file) == forbidden {
				if _, err := os.Stat(filepath.Join(root, file)); errors.Is(err, os.ErrNotExist) {
					break
				} else if err != nil {
					return err
				}
				return fmt.Errorf("forbidden public path is tracked: %s", file)
			}
		}
		if strings.HasPrefix(filepath.ToSlash(file), ".private/") || strings.HasPrefix(filepath.ToSlash(file), "docs/private/") {
			if _, err := os.Stat(filepath.Join(root, file)); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				return err
			}
			return fmt.Errorf("forbidden private path is tracked: %s", file)
		}
		if !isTextCheckedFile(file) {
			continue
		}

		content, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		text := string(content)
		patterns := forbiddenSecretPatterns
		if isDocLikeFile(file) {
			patterns = append(append([]*regexp.Regexp(nil), forbiddenSecretPatterns...), forbiddenDocPatterns...)
		}

		for _, pattern := range patterns {
			if pattern.MatchString(text) {
				return fmt.Errorf("%s contains a public-repo forbidden pattern: %s", file, pattern.String())
			}
		}
	}

	return nil
}

func trackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, errors.New(message)
	}

	trimmed := bytes.TrimRight(stdout.Bytes(), "\x00")
	if len(trimmed) == 0 {
		return nil, nil
	}
	parts := bytes.Split(trimmed, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		files = append(files, string(part))
	}
	return files, nil
}

func isTextCheckedFile(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	if ext == ".md" || ext == ".go" || ext == ".sh" || ext == ".yml" || ext == ".yaml" || ext == ".json" || ext == ".txt" {
		return true
	}
	return base == ".gitignore" || base == "Makefile" || base == "LICENSE"
}

func isDocLikeFile(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	return ext == ".md" || ext == ".txt" || ext == ".yml" || ext == ".yaml" || ext == ".json" || ext == ".sh" || base == ".gitignore" || base == "Makefile" || base == "LICENSE"
}

func extractFrontmatter(content string) (string, error) {
	content = runtime.StripBOM(content)
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return "", errors.New("missing YAML frontmatter")
	}

	trimmed := strings.TrimPrefix(content, "---\r\n")
	trimmed = strings.TrimPrefix(trimmed, "---\n")
	endMarker := "\n---\n"
	if strings.Contains(trimmed, "\r\n") {
		endMarker = "\r\n---\r\n"
	}

	parts := strings.SplitN(trimmed, endMarker, 2)
	if len(parts) != 2 {
		return "", errors.New("unterminated YAML frontmatter")
	}

	return parts[0], nil
}
