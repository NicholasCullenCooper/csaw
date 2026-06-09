package mcpmerge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a tiny test helper.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadFragmentParsesAndExtractsRawText(t *testing.T) {
	tmp := t.TempDir()
	fragPath := filepath.Join(tmp, "codex.toml")
	writeFile(t, fragPath, `# Top-level comment in source fragment
[mcp_servers.github]
# Server-specific comment
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
env_vars = ["GITHUB_TOKEN"]

[mcp_servers.linear]
command = "npx"
args = ["-y", "@tacticiq/linear-mcp"]
env_vars = ["LINEAR_API_KEY"]
`)

	target := KnownMergeTargets[0] // codex
	frag, err := ReadFragment(target, fragPath, "team")
	if err != nil {
		t.Fatalf("ReadFragment: %v", err)
	}
	if frag.SourceName != "team" {
		t.Errorf("SourceName = %q", frag.SourceName)
	}
	if len(frag.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(frag.Servers))
	}
	if frag.Servers[0].Name != "github" {
		t.Errorf("first server name = %q (servers should be sorted)", frag.Servers[0].Name)
	}
	// Raw text must preserve the server comment.
	if !strings.Contains(frag.Servers[0].RawTOML, "# Server-specific comment") {
		t.Errorf("github server raw text lost its comment: %q", frag.Servers[0].RawTOML)
	}
}

func TestReadFragmentRejectsLiteralSecrets(t *testing.T) {
	tmp := t.TempDir()
	fragPath := filepath.Join(tmp, "codex.toml")
	writeFile(t, fragPath, `[mcp_servers.bad]
command = "npx"
token = "ghp_literalsecretwouldgohere"
`)
	target := KnownMergeTargets[0]
	_, err := ReadFragment(target, fragPath, "team")
	if err == nil {
		t.Fatal("expected secret-validation error, got nil")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error %q should mention token", err.Error())
	}
}

func TestReadFragmentRequiresMCPServers(t *testing.T) {
	tmp := t.TempDir()
	fragPath := filepath.Join(tmp, "codex.toml")
	writeFile(t, fragPath, `# fragment with no mcp_servers
[other]
x = 1
`)
	target := KnownMergeTargets[0]
	_, err := ReadFragment(target, fragPath, "team")
	if err == nil || !strings.Contains(err.Error(), "no [mcp_servers.*] tables") {
		t.Fatalf("expected 'no mcp_servers' error, got: %v", err)
	}
}

func TestPlanMergeFreshTarget(t *testing.T) {
	tmp := t.TempDir()
	// Source fragment.
	srcDir := filepath.Join(tmp, "src")
	fragPath := filepath.Join(srcDir, "codex.toml")
	writeFile(t, fragPath, `[mcp_servers.github]
command = "npx"
env_vars = ["GITHUB_TOKEN"]
`)
	frag, err := ReadFragment(KnownMergeTargets[0], fragPath, "team")
	if err != nil {
		t.Fatal(err)
	}

	// Project root with no existing target file.
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanMerge(KnownMergeTargets[0], projectRoot, frag)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetExists {
		t.Error("TargetExists should be false for fresh project")
	}
	if plan.HasManagedSection {
		t.Error("HasManagedSection should be false for fresh project")
	}
	if len(plan.WillApply) != 1 {
		t.Errorf("WillApply = %d, want 1", len(plan.WillApply))
	}
	if len(plan.Conflicts) != 0 {
		t.Errorf("Conflicts = %v, want none", plan.Conflicts)
	}
}

func TestPlanMergeDetectsUserConflict(t *testing.T) {
	tmp := t.TempDir()
	// Project already has [mcp_servers.github] defined by the user.
	projectRoot := filepath.Join(tmp, "project")
	writeFile(t, filepath.Join(projectRoot, ".codex", "config.toml"), `model = "gpt-4o"

[mcp_servers.github]
command = "user-defined"
env_vars = ["MY_GH_TOKEN"]
`)

	// Source fragment also defines github.
	fragPath := filepath.Join(tmp, "src", "codex.toml")
	writeFile(t, fragPath, `[mcp_servers.github]
command = "npx"
env_vars = ["GITHUB_TOKEN"]

[mcp_servers.linear]
command = "npx"
env_vars = ["LINEAR_API_KEY"]
`)
	frag, err := ReadFragment(KnownMergeTargets[0], fragPath, "team")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanMerge(KnownMergeTargets[0], projectRoot, frag)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.TargetExists {
		t.Fatal("TargetExists should be true")
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].ServerName != "github" {
		t.Errorf("expected one github conflict, got %v", plan.Conflicts)
	}
	if len(plan.WillApply) != 1 || plan.WillApply[0].Name != "linear" {
		t.Errorf("expected linear to apply, got %v", plan.WillApply)
	}
}

func TestApplyAndRemoveRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")
	projectRoot := filepath.Join(tmp, "project")

	// Existing user config.
	targetPath := filepath.Join(projectRoot, ".codex", "config.toml")
	originalUserContent := `# User-managed Codex config
model = "gpt-4o"

# User's own server (with comment)
[mcp_servers.user_thing]
command = "echo"
args = ["hello"]

[providers.openai]
base_url = "https://api.openai.com/v1"
`
	writeFile(t, targetPath, originalUserContent)

	// Source fragment.
	fragPath := filepath.Join(tmp, "src", "codex.toml")
	writeFile(t, fragPath, `[mcp_servers.github]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
env_vars = ["GITHUB_TOKEN"]
`)
	frag, err := ReadFragment(KnownMergeTargets[0], fragPath, "team")
	if err != nil {
		t.Fatal(err)
	}

	// Apply.
	plan, err := PlanMerge(KnownMergeTargets[0], projectRoot, frag)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, stateDir); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Verify resulting file: user content intact + managed section appended.
	after, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	afterStr := string(after)
	if !strings.HasPrefix(afterStr, "# User-managed Codex config") {
		t.Errorf("preamble lost: %q", afterStr[:50])
	}
	if !strings.Contains(afterStr, "# User's own server (with comment)") {
		t.Error("user's comment lost — bounded-section approach failed")
	}
	if !strings.Contains(afterStr, "[mcp_servers.user_thing]") {
		t.Error("user's server lost")
	}
	if !strings.Contains(afterStr, "[mcp_servers.github]") {
		t.Error("csaw's server not added")
	}

	// Re-apply should be idempotent (same content).
	plan2, _ := PlanMerge(KnownMergeTargets[0], projectRoot, frag)
	if _, err := Apply(plan2, stateDir); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	after2, _ := os.ReadFile(targetPath)
	if string(after2) != string(after) {
		t.Error("re-apply produced different content (not idempotent)")
	}

	// Remove and verify we're back to the original.
	if _, err := Remove(KnownMergeTargets[0], projectRoot, stateDir); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	final, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(final) != originalUserContent {
		t.Errorf("after Remove, file differs from original.\nWant:\n%s\nGot:\n%s", originalUserContent, string(final))
	}
}

func TestRemoveDetectsDrift(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "state")
	projectRoot := filepath.Join(tmp, "project")
	targetPath := filepath.Join(projectRoot, ".codex", "config.toml")
	writeFile(t, targetPath, "model = \"gpt-4o\"\n")

	fragPath := filepath.Join(tmp, "src", "codex.toml")
	writeFile(t, fragPath, `[mcp_servers.x]
command = "y"
env_vars = ["Z"]
`)
	frag, _ := ReadFragment(KnownMergeTargets[0], fragPath, "team")
	plan, _ := PlanMerge(KnownMergeTargets[0], projectRoot, frag)
	if _, err := Apply(plan, stateDir); err != nil {
		t.Fatal(err)
	}

	// Tamper with the managed section.
	current, _ := os.ReadFile(targetPath)
	tampered := strings.Replace(string(current), "command = \"y\"", "command = \"z-tampered\"", 1)
	if tampered == string(current) {
		t.Fatal("test setup: tamper didn't change content")
	}
	if err := os.WriteFile(targetPath, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Remove(KnownMergeTargets[0], projectRoot, stateDir)
	if err == nil {
		t.Fatal("expected drift-detected error after tampering")
	}
	if !strings.Contains(err.Error(), "SHA mismatch") {
		t.Errorf("error should mention SHA mismatch; got %q", err.Error())
	}
}
