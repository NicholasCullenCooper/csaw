package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// e2eEnv sets up an isolated csaw environment for end-to-end testing.
// It creates a temporary CSAW_HOME, a git-initialized project directory,
// and returns a cleanup function.
type e2eEnv struct {
	t          *testing.T
	csawHome   string
	projectDir string
	oldHome    string
	oldDir     string
}

func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if canSymlink, reason := canCreateDiffSymlink(); !canSymlink {
		t.Skip(reason)
	}

	root := t.TempDir()
	csawHome := filepath.Join(root, ".csaw")
	projectDir := filepath.Join(root, "project")

	os.MkdirAll(projectDir, 0o755)

	// Initialize a git repo in the project (required for csaw mount)
	cmd := exec.Command("git", "init")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Set CSAW_HOME to isolate from real config
	oldHome := os.Getenv("CSAW_HOME")
	os.Setenv("CSAW_HOME", csawHome)

	// Replace stdin with a pipe so isInteractive() returns false.
	// This prevents bubbletea TUI flows from hanging in CI.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.Close() // close write end immediately → reads return EOF
	os.Stdin = r

	// cd into the project
	oldDir, _ := os.Getwd()
	os.Chdir(projectDir)

	t.Cleanup(func() {
		os.Chdir(oldDir)
		os.Stdin = oldStdin
		r.Close()
		if oldHome != "" {
			os.Setenv("CSAW_HOME", oldHome)
		} else {
			os.Unsetenv("CSAW_HOME")
		}
	})

	return &e2eEnv{
		t:          t,
		csawHome:   csawHome,
		projectDir: projectDir,
		oldHome:    oldHome,
		oldDir:     oldDir,
	}
}

// run executes a csaw subcommand and returns stdout. Fails the test on error.
func (e *e2eEnv) run(args ...string) string {
	e.t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		e.t.Fatalf("csaw %s failed: %v\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// runExpectError executes a csaw subcommand and expects it to fail.
func (e *e2eEnv) runExpectError(args ...string) string {
	e.t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err == nil {
		e.t.Fatalf("csaw %s should have failed but succeeded\nstdout: %s",
			strings.Join(args, " "), stdout.String())
	}
	return stderr.String()
}

// createRegistry creates a registry directory with given files.
// Files map relative path → content.
func (e *e2eEnv) createRegistry(dir string, files map[string]string) string {
	e.t.Helper()
	absDir := filepath.Join(filepath.Dir(e.projectDir), dir)
	os.MkdirAll(absDir, 0o755)

	for relPath, content := range files {
		fullPath := filepath.Join(absDir, filepath.FromSlash(relPath))
		os.MkdirAll(filepath.Dir(fullPath), 0o755)
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			e.t.Fatalf("WriteFile(%s) error = %v", relPath, err)
		}
	}

	// git init for push compatibility
	cmd := exec.Command("git", "init")
	cmd.Dir = absDir
	cmd.CombinedOutput()

	return absDir
}

// fileExists checks if a file exists in the project directory.
func (e *e2eEnv) fileExists(relPath string) bool {
	_, err := os.Stat(filepath.Join(e.projectDir, filepath.FromSlash(relPath)))
	return err == nil
}

// readFile reads a file from the project directory.
func (e *e2eEnv) readFile(relPath string) string {
	e.t.Helper()
	content, err := os.ReadFile(filepath.Join(e.projectDir, filepath.FromSlash(relPath)))
	if err != nil {
		e.t.Fatalf("readFile(%s) error = %v", relPath, err)
	}
	return string(content)
}

// --- End-to-end tests ---

func TestE2EInitSourceAddMountUnmount(t *testing.T) {
	env := newE2EEnv(t)

	// 1. Init a registry
	registryDir := filepath.Join(filepath.Dir(env.projectDir), "my-registry")
	env.run("init", registryDir, "--name", "test")

	// Verify scaffolding
	if _, err := os.Stat(filepath.Join(registryDir, "csaw.yml")); err != nil {
		t.Fatal("csaw.yml not created")
	}
	if _, err := os.Stat(filepath.Join(registryDir, "AGENTS.md")); err != nil {
		t.Fatal("AGENTS.md not created")
	}
	if _, err := os.Stat(filepath.Join(registryDir, "skills", "commit-message", "SKILL.md")); err != nil {
		t.Fatal("starter skill not created")
	}

	// 2. Source add
	env.run("source", "add", "test", registryDir)

	// Verify source is registered
	out := env.run("source", "list")
	if !strings.Contains(out, "test") {
		t.Fatalf("source list should contain 'test', got: %s", out)
	}

	// 3. Mount
	env.run("mount", "--profile", "test/default")

	// Verify AGENTS.md is mounted at project root
	if !env.fileExists("AGENTS.md") {
		t.Fatal("AGENTS.md not mounted")
	}
	content := env.readFile("AGENTS.md")
	if !strings.Contains(content, "Agent Instructions") {
		t.Fatalf("AGENTS.md content unexpected: %s", content)
	}

	// Verify skills are mounted into .agents/skills/ (fallback tool dir)
	if !env.fileExists(".agents/skills/commit-message/SKILL.md") {
		t.Fatal("commit-message skill not mounted to .agents/skills/")
	}

	// 4. Inspect
	env.run("check")

	// 5. Unmount
	env.run("unmount")

	// Verify AGENTS.md is gone
	if env.fileExists("AGENTS.md") {
		t.Fatal("AGENTS.md should be removed after unmount")
	}
}

func TestE2EMountReplacesExisting(t *testing.T) {
	env := newE2EEnv(t)

	// Create two registries with different content
	reg1 := env.createRegistry("reg1", map[string]string{
		"csaw.yml":  "default:\n  include:\n    - AGENTS.md\n",
		"AGENTS.md": "registry one",
	})
	reg2 := env.createRegistry("reg2", map[string]string{
		"csaw.yml":  "default:\n  include:\n    - AGENTS.md\n",
		"AGENTS.md": "registry two",
	})

	env.run("source", "add", "reg1", reg1)
	env.run("source", "add", "reg2", reg2, "--priority", "5")

	// Mount reg1
	env.run("mount", "--profile", "reg1/default")
	if env.readFile("AGENTS.md") != "registry one" {
		t.Fatal("expected registry one content")
	}

	// Mount reg2 — should auto-unmount reg1
	env.run("mount", "--profile", "reg2/default")
	if env.readFile("AGENTS.md") != "registry two" {
		t.Fatal("expected registry two content after remount")
	}
}

func TestE2ESourcePriority(t *testing.T) {
	env := newE2EEnv(t)

	// Both registries provide AGENTS.md — higher priority should win
	regLow := env.createRegistry("low", map[string]string{
		"csaw.yml":  "all:\n  include:\n    - AGENTS.md\n",
		"AGENTS.md": "low priority",
	})
	regHigh := env.createRegistry("high", map[string]string{
		"csaw.yml":  "all:\n  include:\n    - AGENTS.md\n",
		"AGENTS.md": "high priority",
	})

	env.run("source", "add", "low", regLow, "--priority", "0")
	env.run("source", "add", "high", regHigh, "--priority", "10")

	// Mount from both sources — priority should pick "high"
	env.run("mount", "AGENTS.md")
	if env.readFile("AGENTS.md") != "high priority" {
		t.Fatalf("expected high priority content, got: %s", env.readFile("AGENTS.md"))
	}
}

func TestE2EStashAndRestore(t *testing.T) {
	env := newE2EEnv(t)

	// Create a local file that will be overwritten
	localFile := filepath.Join(env.projectDir, "AGENTS.md")
	os.WriteFile(localFile, []byte("local content"), 0o644)

	reg := env.createRegistry("reg", map[string]string{
		"csaw.yml":  "default:\n  include:\n    - AGENTS.md\n",
		"AGENTS.md": "registry content",
	})

	env.run("source", "add", "reg", reg)
	env.run("mount", "--profile", "reg/default", "--force")

	// Should be registry content now
	if env.readFile("AGENTS.md") != "registry content" {
		t.Fatal("expected registry content after mount")
	}

	// Unmount should restore original
	env.run("unmount")
	if env.readFile("AGENTS.md") != "local content" {
		t.Fatalf("expected local content after unmount, got: %s", env.readFile("AGENTS.md"))
	}
}

func TestE2EMountRestore(t *testing.T) {
	env := newE2EEnv(t)

	reg := env.createRegistry("reg", map[string]string{
		"csaw.yml":  "default:\n  include:\n    - AGENTS.md\n",
		"AGENTS.md": "hello",
	})

	env.run("source", "add", "reg", reg)
	env.run("mount", "--profile", "reg/default")
	env.run("unmount")

	// Restore should remount
	env.run("mount", "--restore")
	if !env.fileExists("AGENTS.md") {
		t.Fatal("AGENTS.md should be remounted after --restore")
	}
}

func TestE2EFork(t *testing.T) {
	env := newE2EEnv(t)

	regTeam := env.createRegistry("team", map[string]string{
		"csaw.yml":  "default:\n  include:\n    - AGENTS.md\n",
		"AGENTS.md": "team rules",
	})
	regPersonal := env.createRegistry("personal", map[string]string{
		"csaw.yml": "default:\n  include:\n    - \"**/*\"\n",
	})

	env.run("source", "add", "team", regTeam)
	env.run("source", "add", "personal", regPersonal, "--priority", "10")

	// Fork team's AGENTS.md into personal
	env.run("fork", "team/AGENTS.md", "--into", "personal")

	// Verify the file was copied to personal registry
	forkedPath := filepath.Join(filepath.Dir(env.projectDir), "personal", "AGENTS.md")
	content, err := os.ReadFile(forkedPath)
	if err != nil {
		t.Fatalf("forked file not found: %v", err)
	}
	if string(content) != "team rules" {
		t.Fatalf("forked content = %q, want %q", string(content), "team rules")
	}
}

func TestE2EInitAdopt(t *testing.T) {
	env := newE2EEnv(t)

	// Place some AI config files in the project
	os.MkdirAll(filepath.Join(env.projectDir, ".claude", "skills", "testing"), 0o755)
	os.WriteFile(filepath.Join(env.projectDir, "AGENTS.md"), []byte("project agents"), 0o644)
	os.WriteFile(filepath.Join(env.projectDir, ".claude", "skills", "testing", "SKILL.md"), []byte("test skill"), 0o644)

	registryDir := filepath.Join(filepath.Dir(env.projectDir), "adopted-registry")
	env.run("init", registryDir, "--adopt")

	// Verify skill was adopted (AGENTS.md already exists as starter, so project's is skipped)
	skillPath := filepath.Join(registryDir, "skills", "testing", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("adopted skill not found: %v", err)
	}
	if string(content) != "test skill" {
		t.Fatalf("adopted skill content = %q, want %q", string(content), "test skill")
	}
}

func TestE2EVersionCommand(t *testing.T) {
	env := newE2EEnv(t)
	out := env.run("version")
	if !strings.Contains(out, "dev") {
		// In test builds, version is "dev" (the default from root.go)
		t.Fatalf("version output = %q, expected to contain 'dev'", out)
	}
}
