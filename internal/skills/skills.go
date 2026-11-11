// Package skills validates skill directories against the Agent Skills
// specification (agentskills.io): a directory name of 1 to 64 lowercase
// alphanumeric characters and single hyphens, and a SKILL.md whose
// frontmatter `name`, when present, equals the directory name. Both Claude
// Code and Codex skip skills that break these rules, so csaw refuses to
// mount or export them rather than let them fail silently.
package skills

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const MaxNameLength = 64

var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateName checks a skill directory name.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is empty")
	}
	if len(name) > MaxNameLength {
		return fmt.Errorf("skill name %q is longer than %d characters", name, MaxNameLength)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("skill name %q must be lowercase letters, digits, and single hyphens (no leading, trailing, or double hyphens)", name)
	}
	return nil
}

// FrontmatterName returns the `name` field of a SKILL.md's YAML frontmatter,
// or "" when the file has no frontmatter or no name field.
func FrontmatterName(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	first := true
	inFrontmatter := false
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			if strings.TrimSpace(strings.TrimPrefix(line, "\ufeff")) != "---" {
				return "", nil
			}
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			break
		}
		if strings.TrimSpace(line) == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "name" {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"'`), nil
	}
	return "", scanner.Err()
}

// Validate checks a skill directory name and, when skillMD exists, that its
// frontmatter name matches.
func Validate(name string, skillMD string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if skillMD == "" {
		return nil
	}
	declared, err := FrontmatterName(skillMD)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if declared != "" && declared != name {
		return fmt.Errorf("skill %q declares name %q in SKILL.md frontmatter; the two must match", name, declared)
	}
	return nil
}
