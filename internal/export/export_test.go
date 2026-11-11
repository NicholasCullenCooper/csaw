package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NicholasCullenCooper/csaw/internal/mount"
)

func fixtureRegistry(t *testing.T) (string, []mount.SourceEntry) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"AGENTS.md":                     "# instructions",
		"rules/go.md":                   "rule",
		"agents/reviewer.md":            "---\nname: reviewer\n---\nreview",
		"skills/deploy/SKILL.md":        "---\nname: deploy\ndescription: d\n---\nsee references/r.md",
		"skills/deploy/references/r.md": "runbook",
		"skills/Bad Name/SKILL.md":      "---\nname: bad\n---\n",
		"hooks/hooks.json":              `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"scripts/guard.sh"}]}]}}`,
		"hooks/scripts/guard.sh":        "#!/bin/sh\n",
		"mcp/claude-code.json":          `{"mcpServers":{"docs":{"command":"npx","args":["-y","docs-mcp"],"env":{"DOCS_TOKEN":"$DOCS_TOKEN"}},"remote":{"type":"http","url":"https://mcp.example.com/sse","headers":{"X-Team":"a"}}}}`,
		"mcp/codex.toml":                "[mcp_servers.docs]\ncommand = \"npx\"\nargs = [\"-y\", \"docs-mcp\"]\nenv_vars = [\"DOCS_TOKEN\"]\n\n[mcp_servers.remote]\nurl = \"https://mcp.example.com/mcp\"\n",
		"ignore/cursor":                 "node_modules",
	}
	var entries []mount.SourceEntry
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
		entries = append(entries, mount.SourceEntry{SourceName: "team", RelativePath: rel, QualifiedPath: "team/" + rel, FullPath: full})
	}
	return root, entries
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

func TestPluginClaudeCode(t *testing.T) {
	_, entries := fixtureRegistry(t)
	out := filepath.Join(t.TempDir(), "plugin")

	result, err := Plugin(FormatClaudeCode, Input{Name: "team-tools", Version: "1.2.3", Description: "team plugin", Entries: entries}, out, false)
	if err != nil {
		t.Fatalf("Plugin() error = %v", err)
	}

	manifest := readJSON(t, filepath.Join(out, ".claude-plugin", "plugin.json"))
	if manifest["name"] != "team-tools" || manifest["version"] != "1.2.3" || manifest["description"] != "team plugin" {
		t.Fatalf("manifest = %v", manifest)
	}
	for _, want := range []string{
		"skills/deploy/SKILL.md", "skills/deploy/references/r.md",
		"agents/reviewer.md", "hooks/hooks.json", "scripts/guard.sh", ".mcp.json",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(want))); err != nil {
			t.Errorf("%s should be written: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "skills", "Bad Name")); err == nil {
		t.Error("invalid skill must not be exported")
	}
	hooksDoc := readJSON(t, filepath.Join(out, "hooks", "hooks.json"))
	if !strings.Contains(string(mustMarshal(hooksDoc)), "${CLAUDE_PLUGIN_ROOT}/scripts/guard.sh") {
		t.Fatalf("hook command should point at the plugin's scripts dir: %v", hooksDoc)
	}
	omitted := strings.Join(result.Omitted, "\n")
	for _, want := range []string{"Bad Name", "rules omitted", "instructions omitted", "ignore files omitted", "codex.toml"} {
		if !strings.Contains(omitted, want) {
			t.Errorf("omitted notes missing %q:\n%s", want, omitted)
		}
	}

	if _, err := Plugin(FormatClaudeCode, Input{Name: "team-tools", Entries: entries}, out, false); err == nil {
		t.Fatal("non-empty out dir should be refused without force")
	}
	if _, err := Plugin(FormatClaudeCode, Input{Name: "team-tools", Entries: entries}, out, true); err != nil {
		t.Fatalf("force should replace: %v", err)
	}
}

func TestPluginAgentPluginsPrefersCodexMCP(t *testing.T) {
	_, entries := fixtureRegistry(t)
	out := filepath.Join(t.TempDir(), "plugin")

	result, err := Plugin(FormatAgentPlugins, Input{Name: "team-tools", Entries: entries}, out, false)
	if err != nil {
		t.Fatalf("Plugin() error = %v", err)
	}
	manifest := readJSON(t, filepath.Join(out, "plugin.json"))
	if manifest["$schema"] != agentPluginsSchema || manifest["name"] != "team-tools" {
		t.Fatalf("manifest = %v", manifest)
	}
	mcp := readJSON(t, filepath.Join(out, "mcp.json"))
	servers := mcp["mcpServers"].(map[string]any)
	docs := servers["docs"].(map[string]any)
	if docs["type"] != "stdio" || docs["command"] != "npx" {
		t.Fatalf("docs server = %v", docs)
	}
	remote := servers["remote"].(map[string]any)
	if remote["type"] != "streamable-http" || remote["url"] != "https://mcp.example.com/mcp" {
		t.Fatalf("remote server = %v", remote)
	}
	for _, absent := range []string{"agents", "hooks", "scripts", ".mcp.json", ".claude-plugin"} {
		if _, err := os.Stat(filepath.Join(out, absent)); err == nil {
			t.Errorf("%s is outside the Agent Plugins format and must not be written", absent)
		}
	}
	omitted := strings.Join(result.Omitted, "\n")
	for _, want := range []string{"env_vars", "agents omitted", "hooks omitted"} {
		if !strings.Contains(omitted, want) {
			t.Errorf("omitted notes missing %q:\n%s", want, omitted)
		}
	}
}

func TestPluginAgentPluginsFallsBackToClaudeMCP(t *testing.T) {
	root, entries := fixtureRegistry(t)
	os.Remove(filepath.Join(root, "mcp", "codex.toml"))
	var kept []mount.SourceEntry
	for _, e := range entries {
		if e.RelativePath != "mcp/codex.toml" {
			kept = append(kept, e)
		}
	}
	out := filepath.Join(t.TempDir(), "plugin")
	if _, err := Plugin(FormatAgentPlugins, Input{Name: "x", Entries: kept}, out, false); err != nil {
		t.Fatalf("Plugin() error = %v", err)
	}
	servers := readJSON(t, filepath.Join(out, "mcp.json"))["mcpServers"].(map[string]any)
	remote := servers["remote"].(map[string]any)
	if remote["type"] != "streamable-http" || remote["headers"].(map[string]any)["X-Team"] != "a" {
		t.Fatalf("remote from claude-code.json = %v", remote)
	}
	docs := servers["docs"].(map[string]any)
	if docs["env"].(map[string]any)["DOCS_TOKEN"] != "$DOCS_TOKEN" {
		t.Fatalf("env reference should pass through: %v", docs)
	}
}

func TestPluginRefusesLiteralSecrets(t *testing.T) {
	root := t.TempDir()
	mcp := filepath.Join(root, "mcp", "claude-code.json")
	os.MkdirAll(filepath.Dir(mcp), 0o755)
	os.WriteFile(mcp, []byte(`{"mcpServers":{"docs":{"command":"x","env":{"API_TOKEN":"sk-live-123"}}}}`), 0o644)
	entries := []mount.SourceEntry{{SourceName: "t", RelativePath: "mcp/claude-code.json", FullPath: mcp}}
	for _, format := range []Format{FormatClaudeCode, FormatAgentPlugins} {
		if _, err := Plugin(format, Input{Name: "x", Entries: entries}, filepath.Join(t.TempDir(), "o"), false); err == nil || !strings.Contains(err.Error(), "API_TOKEN") {
			t.Errorf("%s: literal secret should be refused, got %v", format, err)
		}
	}
}

func TestValidateNameAndMarketplace(t *testing.T) {
	for _, name := range []string{"team-tools", "a.b", "x1"} {
		if err := ValidateName(FormatAgentPlugins, name); err != nil {
			t.Errorf("%q should be valid: %v", name, err)
		}
	}
	for _, name := range []string{"", "Team", "a--b", "-a", "a.", "a b"} {
		if err := ValidateName(FormatAgentPlugins, name); err == nil {
			t.Errorf("%q should be invalid for Agent Plugins", name)
		}
	}
	if err := ValidateName(FormatClaudeCode, "Team Tools"); err == nil {
		t.Error("spaces are invalid for Claude Code plugin names")
	}
	if err := ValidateName(FormatClaudeCode, "Team.Tools"); err != nil {
		t.Errorf("Claude Code allows mixed case and periods: %v", err)
	}

	out := t.TempDir()
	err := Marketplace("team", "", []MarketplaceEntry{{Name: "team-backend", Source: "./plugins/team-backend", Description: "backend"}}, out)
	if err != nil {
		t.Fatalf("Marketplace() error = %v", err)
	}
	doc := readJSON(t, filepath.Join(out, ".claude-plugin", "marketplace.json"))
	if doc["name"] != "team" || doc["owner"].(map[string]any)["name"] != "csaw" {
		t.Fatalf("marketplace = %v", doc)
	}
	plugins := doc["plugins"].([]any)
	if len(plugins) != 1 || plugins[0].(map[string]any)["source"] != "./plugins/team-backend" {
		t.Fatalf("plugins = %v", plugins)
	}
	if err := Marketplace("has space", "", nil, out); err == nil {
		t.Error("marketplace names cannot contain spaces")
	}
	if err := Marketplace("ok", "", []MarketplaceEntry{{Name: "x", Source: "./../x"}}, out); err == nil {
		t.Error("sources must not escape the marketplace")
	}
}

func mustMarshal(doc map[string]any) []byte {
	content, _ := json.Marshal(doc)
	return content
}
