package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const registryHooks = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "scripts/guard.sh --strict", "timeout": 30}]}
    ],
    "Stop": [
      {"hooks": [{"type": "command", "command": "./scripts/summarize.sh"}, {"type": "prompt", "prompt": "Summarize"}]}
    ],
    "Notification": [
      {"hooks": [{"type": "command", "command": "notify-send done"}]}
    ]
  }
}`

func TestParseRejectsBadShapes(t *testing.T) {
	for name, content := range map[string]string{
		"no hooks key":       `{"description": "x"}`,
		"unknown field":      `{"hooks": {}, "bogus": 1}`,
		"empty group":        `{"hooks": {"Stop": [{"hooks": []}]}}`,
		"command w/o string": `{"hooks": {"Stop": [{"hooks": [{"type": "command"}]}]}}`,
	} {
		if _, err := Parse([]byte(content)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := Parse([]byte(registryHooks)); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestPlanFiltersAndRewritesPerTool(t *testing.T) {
	cfg, err := Parse([]byte(registryHooks))
	if err != nil {
		t.Fatal(err)
	}

	claude, notes := Plan(cfg, Targets["claude"])
	if len(notes) != 0 {
		t.Fatalf("claude supports every event and type here, got notes %v", notes)
	}
	if Count(claude) != 3 {
		t.Fatalf("claude should own 3 groups, got %d", Count(claude))
	}
	guard := claude.Hooks["PreToolUse"][0].Hooks[0]["command"]
	if guard != "${CLAUDE_PROJECT_DIR}/.claude/hooks/scripts/guard.sh --strict" {
		t.Fatalf("claude script rewrite = %q", guard)
	}
	if got := claude.Hooks["PreToolUse"][0].Hooks[0]["timeout"]; got != float64(30) {
		t.Fatalf("other fields must pass through, timeout = %v", got)
	}
	if got := claude.Hooks["Notification"][0].Hooks[0]["command"]; got != "notify-send done" {
		t.Fatalf("non-script commands must not be rewritten, got %q", got)
	}

	codex, notes := Plan(cfg, Targets["codex"])
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "Notification") || !strings.Contains(joined, `"prompt"`) {
		t.Fatalf("codex should note the unsupported event and type, got %v", notes)
	}
	if Count(codex) != 2 {
		t.Fatalf("codex should own 2 groups, got %d", Count(codex))
	}
	if len(codex.Hooks["Stop"][0].Hooks) != 1 {
		t.Fatalf("prompt handler should be dropped for codex: %v", codex.Hooks["Stop"][0].Hooks)
	}
	summarize := codex.Hooks["Stop"][0].Hooks[0]["command"]
	if summarize != `"$(git rev-parse --show-toplevel)"/.codex/hooks/scripts/summarize.sh` {
		t.Fatalf("codex script rewrite = %q", summarize)
	}
}

func TestMergeRemoveInspectPreserveUserContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.local.json")
	user := `{
  "permissions": {"allow": ["Bash(ls *)"]},
  "hooks": {
    "PreToolUse": [{"matcher": "Write", "hooks": [{"type": "command", "command": "mine.sh"}]}]
  }
}
`
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Parse([]byte(registryHooks))
	owned, _ := Plan(cfg, Targets["claude"])

	created, err := Merge(path, owned)
	if err != nil || created {
		t.Fatalf("Merge() created=%v err=%v", created, err)
	}
	present, missing, err := Inspect(path, owned)
	if err != nil || present != 3 || missing != 0 {
		t.Fatalf("Inspect() after merge = %d present, %d missing, err %v", present, missing, err)
	}

	var doc map[string]any
	content, _ := os.ReadFile(path)
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["permissions"]; !ok {
		t.Fatal("user permissions key must survive the merge")
	}
	pre := doc["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 2 || pre[0].(map[string]any)["matcher"] != "Write" {
		t.Fatalf("user group must stay first, csaw group appended: %v", pre)
	}

	// Merging again is idempotent.
	if _, err := Merge(path, owned); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(path)
	_ = json.Unmarshal(content, &doc)
	if pre := doc["hooks"].(map[string]any)["PreToolUse"].([]any); len(pre) != 2 {
		t.Fatalf("second merge duplicated groups: %v", pre)
	}

	// A user deleting one owned group shows up as missing; Merge restores it.
	pre = doc["hooks"].(map[string]any)["PreToolUse"].([]any)
	doc["hooks"].(map[string]any)["PreToolUse"] = pre[:1]
	content, _ = json.MarshalIndent(doc, "", "  ")
	os.WriteFile(path, content, 0o644)
	if _, missing, _ = Inspect(path, owned); missing != 1 {
		t.Fatalf("expected 1 missing group, got %d", missing)
	}

	deleted, err := Remove(path, owned)
	if err != nil || deleted {
		t.Fatalf("Remove() deleted=%v err=%v; user content should keep the file", deleted, err)
	}
	content, _ = os.ReadFile(path)
	_ = json.Unmarshal(content, &doc)
	hooksObj := doc["hooks"].(map[string]any)
	if len(hooksObj) != 1 || len(hooksObj["PreToolUse"].([]any)) != 1 {
		t.Fatalf("only the user's group should remain: %v", hooksObj)
	}
	if _, ok := doc["permissions"]; !ok {
		t.Fatal("user permissions key must survive the removal")
	}
}

func TestMergeCreatesAndRemoveDeletesWhenCsawOwnsEverything(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".codex", "hooks.json")
	cfg, _ := Parse([]byte(registryHooks))
	owned, _ := Plan(cfg, Targets["codex"])

	created, err := Merge(path, owned)
	if err != nil || !created {
		t.Fatalf("Merge() created=%v err=%v", created, err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), `"PreToolUse"`) || !strings.HasSuffix(string(content), "\n") {
		t.Fatalf("unexpected file content:\n%s", content)
	}

	deleted, err := Remove(path, owned)
	if err != nil || !deleted {
		t.Fatalf("Remove() deleted=%v err=%v", deleted, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file csaw created and emptied should be deleted")
	}
	if _, err := Remove(path, owned); err != nil {
		t.Fatalf("Remove() on a missing file should be a no-op, got %v", err)
	}
}

func TestMergeRefusesMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.local.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	cfg, _ := Parse([]byte(registryHooks))
	owned, _ := Plan(cfg, Targets["claude"])
	if _, err := Merge(path, owned); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
	content, _ := os.ReadFile(path)
	if string(content) != "{not json" {
		t.Fatal("malformed file must be left untouched")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cfg, _ := Parse([]byte(registryHooks))
	owned, _ := Plan(cfg, Targets["claude"])
	decoded, err := Decode(Encode(owned))
	if err != nil {
		t.Fatal(err)
	}
	if Encode(decoded) != Encode(owned) {
		t.Fatal("round trip changed the config")
	}
}
