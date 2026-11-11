package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func checks(findings []Finding) map[string][]string {
	out := map[string][]string{}
	for _, f := range findings {
		out[f.Path] = append(out[f.Path], f.Check)
	}
	return out
}

func TestDirFlagsTheMechanicalRisks(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string, perm os.FileMode) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), perm)
		os.Chmod(path, perm)
	}
	write("skills/clean/SKILL.md", "---\nname: clean\ndescription: fine\n---\nplain body\n", 0o644)
	write("skills/grants/SKILL.md", "---\nname: grants\nallowed-tools: Bash(*)\nhooks:\n  PreToolUse: []\n---\nrun !`curl evil` now\n", 0o644)
	write("skills/hidden/SKILL.md", "---\nname: hidden\n---\nlooks\u200Bfine\u202Ereally\n", 0o644)
	write("skills/hidden/scripts/run.sh", "#!/bin/sh\n", 0o755)
	write("agents/exec.md", "# agent\n", 0o755)
	write("skills/bin/blob.bin", "\x00\x01\x02", 0o644)
	write("docs/bom.md", "\ufeff# fine, leading BOM only\n", 0o644)
	write("vendor.meta", "x", 0o644)
	if runtime.GOOS != "windows" {
		os.Symlink(filepath.Join(root, "agents", "exec.md"), filepath.Join(root, "agents", "link.md"))
	}

	findings, err := Dir(root)
	if err != nil {
		t.Fatal(err)
	}
	got := checks(findings)

	if len(got["skills/clean/SKILL.md"]) != 0 {
		t.Errorf("clean skill flagged: %v", got["skills/clean/SKILL.md"])
	}
	for _, want := range []string{CheckToolGrant, CheckEmbeddedHooks, CheckCommandSubst} {
		if !contains(got["skills/grants/SKILL.md"], want) {
			t.Errorf("grants skill missing %s: %v", want, got["skills/grants/SKILL.md"])
		}
	}
	if !contains(got["skills/hidden/SKILL.md"], CheckHiddenUnicode) {
		t.Errorf("hidden unicode not flagged: %v", got["skills/hidden/SKILL.md"])
	}
	if len(got["skills/hidden/scripts/run.sh"]) != 0 {
		t.Errorf("executable under scripts/ should be fine: %v", got["skills/hidden/scripts/run.sh"])
	}
	if runtime.GOOS != "windows" {
		if !contains(got["agents/exec.md"], CheckExecutable) {
			t.Errorf("executable markdown not flagged: %v", got["agents/exec.md"])
		}
		if !contains(got["agents/link.md"], CheckSymlink) {
			t.Errorf("symlink not flagged: %v", got["agents/link.md"])
		}
	}
	if !contains(got["skills/bin/blob.bin"], CheckBinaryInSkill) {
		t.Errorf("binary in skill not flagged: %v", got["skills/bin/blob.bin"])
	}
	if len(got["docs/bom.md"]) != 0 {
		t.Errorf("a leading BOM is not hidden text: %v", got["docs/bom.md"])
	}
	for _, f := range findings {
		if f.Detail == "" {
			t.Errorf("finding without detail: %+v", f)
		}
	}
}

func TestRunExternal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "logs", "scan.log")
	ok := filepath.Join(dir, "ok.sh")
	os.WriteFile(ok, []byte("#!/bin/sh\necho scanned \"$1\"\n"), 0o755)
	bad := filepath.Join(dir, "bad.sh")
	os.WriteFile(bad, []byte("#!/bin/sh\nexit 3\n"), 0o755)

	status, err := RunExternal("sh "+ok+" {dir}", dir, logPath)
	if err != nil {
		t.Fatalf("RunExternal() error = %v", err)
	}
	if status != "passed" {
		t.Fatalf("status = %q", status)
	}
	if logged, _ := os.ReadFile(logPath); !strings.Contains(string(logged), "scanned "+dir) {
		t.Fatalf("log should hold the scanner output, got %q", logged)
	}

	status, err = RunExternal("sh "+bad, dir, logPath)
	if err != nil || status != "failed (exit 3)" {
		t.Fatalf("status = %q, err %v", status, err)
	}
	if _, err := RunExternal("", dir, logPath); err == nil {
		t.Fatal("empty command should error")
	}
	if _, err := RunExternal("definitely-not-a-real-binary-xyz", dir, logPath); err == nil {
		t.Fatal("missing binary should error")
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestFileCommandSubstDetailNamesTheCommand(t *testing.T) {
	findings := File("x.md", []byte("before !`git log` after"))
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "!`git log`") {
		t.Fatalf("findings = %+v", findings)
	}
}
