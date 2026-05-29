package registry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListPresetsContainsExpected(t *testing.T) {
	got := make(map[string]bool)
	for _, p := range ListPresets() {
		got[p.Name] = true
	}
	for _, want := range []string{"solo-engineer", "team-go", "team-frontend"} {
		if !got[want] {
			t.Errorf("ListPresets() missing %q; got %v", want, got)
		}
	}
}

func TestGetPresetUnknownReturnsFalse(t *testing.T) {
	if _, ok := GetPreset("does-not-exist"); ok {
		t.Error("GetPreset(\"does-not-exist\") = ok; want not ok")
	}
}

func TestGetPresetReturnsNonEmpty(t *testing.T) {
	for _, name := range []string{"solo-engineer", "team-go", "team-frontend"} {
		p, ok := GetPreset(name)
		if !ok {
			t.Fatalf("GetPreset(%q) = not ok", name)
		}
		if p.Description == "" {
			t.Errorf("preset %q has empty Description", name)
		}
		if len(p.Files) == 0 {
			t.Errorf("preset %q has no Files", name)
		}
		// Every preset must scaffold the three core files.
		for _, required := range []string{"csaw.yml", ".csawignore", "AGENTS.md"} {
			if _, exists := p.Files[required]; !exists {
				t.Errorf("preset %q missing required file %q", name, required)
			}
		}
	}
}

func TestInitWithPresetWritesPresetFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "go-team-config")
	g := &recordingGit{}

	result, err := Init(context.Background(), g, dir, "", "team-go")
	if err != nil {
		t.Fatalf("Init(... preset=team-go) error = %v", err)
	}

	// Files unique to team-go preset should exist
	for _, rel := range []string{
		"rules/go-conventions.md",
		"rules/security.md",
		"rules/testing.md",
		"agents/code-reviewer.md",
	} {
		path := filepath.Join(result.Path, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("team-go preset missing expected file %s: %v", rel, err)
		}
	}

	// Default-only files should NOT exist (preset replaces them)
	for _, rel := range []string{"skills/code-review/SKILL.md"} {
		path := filepath.Join(result.Path, rel)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("team-go preset should not have written %s (that's a default-only file)", rel)
		}
	}

	// csaw.yml should contain the preset-specific csaw: policy block
	content, err := os.ReadFile(filepath.Join(result.Path, "csaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "csaw:") || !strings.Contains(string(content), "protected:") {
		t.Errorf("team-go csaw.yml missing csaw: policy block; got:\n%s", content)
	}
}

func TestInitWithUnknownPresetReturnsError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "x")
	g := &recordingGit{}

	_, err := Init(context.Background(), g, dir, "", "no-such-preset")
	if err == nil {
		t.Fatal("Init(... preset=no-such-preset) returned nil error; expected failure")
	}
	if !strings.Contains(err.Error(), "no-such-preset") {
		t.Errorf("error should name the bad preset; got: %v", err)
	}
}

func TestInitWithPresetCreatesConventionDirs(t *testing.T) {
	// Even if a preset doesn't seed a file in skills/, the dir should exist
	// so `csaw inspect` / `--kind skills` behaves predictably.
	dir := filepath.Join(t.TempDir(), "solo")
	g := &recordingGit{}

	result, err := Init(context.Background(), g, dir, "", "solo-engineer")
	if err != nil {
		t.Fatal(err)
	}

	for _, sub := range []string{"rules", "agents", "skills"} {
		path := filepath.Join(result.Path, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("convention dir %s not created: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s exists but is not a directory", sub)
		}
	}
}
