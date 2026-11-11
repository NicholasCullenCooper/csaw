package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{"deploy", "code-review", "a1", "x-2-y", strings.Repeat("a", 64)}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("%q should be valid: %v", name, err)
		}
	}
	invalid := []string{"", "Deploy", "code_review", "-deploy", "deploy-", "code--review", "a b", "café", strings.Repeat("a", 65)}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("%q should be invalid", name)
		}
	}
}

func TestFrontmatterNameAndValidate(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte(content), 0o644)
		return path
	}

	matching := write("match.md", "---\nname: deploy\ndescription: d\n---\nbody\n")
	quoted := write("quoted.md", "---\nname: \"deploy\"\n---\n")
	mismatch := write("mismatch.md", "---\nname: other\n---\n")
	none := write("none.md", "# no frontmatter\n")
	bom := write("bom.md", "\ufeff---\nname: deploy\n---\n")

	for _, path := range []string{matching, quoted, bom} {
		if got, err := FrontmatterName(path); err != nil || got != "deploy" {
			t.Errorf("FrontmatterName(%s) = %q, %v", filepath.Base(path), got, err)
		}
	}
	if got, _ := FrontmatterName(none); got != "" {
		t.Errorf("no frontmatter should yield empty name, got %q", got)
	}

	if err := Validate("deploy", matching); err != nil {
		t.Errorf("matching name should validate: %v", err)
	}
	if err := Validate("deploy", none); err != nil {
		t.Errorf("missing frontmatter name is allowed: %v", err)
	}
	if err := Validate("deploy", mismatch); err == nil {
		t.Error("mismatched frontmatter name should fail")
	}
	if err := Validate("deploy", filepath.Join(dir, "absent.md")); err != nil {
		t.Errorf("absent SKILL.md is not this check's problem: %v", err)
	}
	if err := Validate("Bad Name", matching); err == nil {
		t.Error("invalid directory name should fail before reading the file")
	}
}
