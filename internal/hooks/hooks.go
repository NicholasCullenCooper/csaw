// Package hooks projects a registry's hooks/hooks.json into the files Claude
// Code and Codex actually run hooks from. Neither tool scans a directory of
// scripts: Claude Code reads hook declarations from its settings files and
// Codex from .codex/hooks.json, both in the same JSON shape.
//
// csaw owns individual matcher groups inside those files, identified by
// their canonical JSON. Merge appends the groups csaw owns (replacing any
// identical earlier copy), Remove deletes exactly those groups and nothing
// else, and Inspect reports whether they are still present. User-authored
// groups in the same file are never touched.
package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Config is the shape of a registry hooks/hooks.json, shared by Claude Code
// settings files and Codex hooks.json.
type Config struct {
	Description string             `json:"description,omitempty"`
	Hooks       map[string][]Group `json:"hooks"`
}

// Group is one matcher block: an optional matcher and its handlers.
// Handlers are kept as generic objects so every tool-specific field passes
// through untouched.
type Group struct {
	Matcher string           `json:"matcher,omitempty"`
	Hooks   []map[string]any `json:"hooks"`
}

// Target describes one file csaw merges hooks into.
type Target struct {
	// Name is the tool key ("claude", "codex").
	Name string
	// Path is the project-relative file the tool reads hooks from.
	Path string
	// ScriptsDir is where csaw copies hooks/scripts/ for this tool.
	ScriptsDir string
	// Events lists the event names the tool supports.
	Events map[string]bool
	// Types lists the handler types the tool supports.
	Types map[string]bool
	// ScriptCommand turns a registry-relative script path ("scripts/x.sh")
	// into the command string the tool should run.
	ScriptCommand func(rel string) string
}

var claudeEvents = set(
	"SessionStart", "Setup", "UserPromptSubmit", "UserPromptExpansion", "PreToolUse",
	"PermissionRequest", "PermissionDenied", "PostToolUse", "PostToolUseFailure", "PostToolBatch",
	"Notification", "MessageDisplay", "SubagentStart", "SubagentStop", "TaskCreated", "TaskCompleted",
	"Stop", "StopFailure", "TeammateIdle", "InstructionsLoaded", "ConfigChange", "CwdChanged",
	"DirectoryAdded", "FileChanged", "WorktreeCreate", "WorktreeRemove", "PreCompact", "PostCompact",
	"PreModelSwitch", "PostModelSwitch", "Elicitation", "ElicitationResult", "SessionEnd",
)

var codexEvents = set(
	"PreToolUse", "PostToolUse", "PermissionRequest", "PreCompact", "PostCompact", "UserPromptSubmit",
	"SubagentStart", "SubagentStop", "Stop", "SessionStart", "SessionEnd", "Interrupt",
)

// Targets maps tool keys to their hook files. Verified against vendor docs
// on 2026-09-28 (Claude Code hooks reference; Codex hooks reference).
var Targets = map[string]Target{
	"claude": {
		Name:       "claude",
		Path:       ".claude/settings.local.json",
		ScriptsDir: ".claude/hooks/scripts",
		Events:     claudeEvents,
		Types:      set("command", "http", "mcp_tool", "prompt", "agent"),
		ScriptCommand: func(rel string) string {
			return "${CLAUDE_PROJECT_DIR}/.claude/hooks/" + rel
		},
	},
	"codex": {
		Name:       "codex",
		Path:       ".codex/hooks.json",
		ScriptsDir: ".codex/hooks/scripts",
		Events:     codexEvents,
		Types:      set("command", "mcp_tool"),
		// Codex runs command hooks in the session cwd and recommends
		// resolving project-local scripts from the git root.
		ScriptCommand: func(rel string) string {
			return `"$(git rev-parse --show-toplevel)"/.codex/hooks/` + rel
		},
	},
}

func set(items ...string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

// Load parses a registry hooks.json.
func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(content)
}

// Parse validates the registry shape: a top-level "hooks" object whose values
// are arrays of matcher groups with handler objects.
func Parse(content []byte) (Config, error) {
	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("hooks.json: %w", err)
	}
	if cfg.Hooks == nil {
		return Config{}, errors.New(`hooks.json: missing top-level "hooks" object`)
	}
	for event, groups := range cfg.Hooks {
		for i, group := range groups {
			if len(group.Hooks) == 0 {
				return Config{}, fmt.Errorf("hooks.json: %s[%d] has no handlers", event, i)
			}
			for j, handler := range group.Hooks {
				if handlerType(handler) == "command" {
					if _, ok := handler["command"].(string); !ok {
						return Config{}, fmt.Errorf("hooks.json: %s[%d].hooks[%d] is a command hook without a command", event, i, j)
					}
				}
			}
		}
	}
	return cfg, nil
}

func handlerType(handler map[string]any) string {
	if value, ok := handler["type"].(string); ok && value != "" {
		return value
	}
	return "command"
}

// Plan derives the groups csaw will own in target from a registry config:
// unsupported events and handler types are dropped with a note, and script
// commands are rewritten to the tool's copied script location.
func Plan(cfg Config, target Target) (owned Config, notes []string) {
	owned = Config{Hooks: map[string][]Group{}}
	for _, event := range sortedEvents(cfg.Hooks) {
		if !target.Events[event] {
			notes = append(notes, fmt.Sprintf("%s: event %s is not supported by %s; skipped", target.Path, event, target.Name))
			continue
		}
		for _, group := range cfg.Hooks[event] {
			var handlers []map[string]any
			for _, handler := range group.Hooks {
				kind := handlerType(handler)
				if !target.Types[kind] {
					notes = append(notes, fmt.Sprintf("%s: %s hook type %q is not supported by %s; skipped", target.Path, event, kind, target.Name))
					continue
				}
				handlers = append(handlers, rewriteScript(handler, target))
			}
			if len(handlers) == 0 {
				continue
			}
			owned.Hooks[event] = append(owned.Hooks[event], Group{Matcher: group.Matcher, Hooks: handlers})
		}
	}
	return owned, notes
}

// rewriteScript points a "scripts/..." command at the tool's copied script.
func rewriteScript(handler map[string]any, target Target) map[string]any {
	out := make(map[string]any, len(handler))
	for key, value := range handler {
		out[key] = value
	}
	command, ok := out["command"].(string)
	if !ok || handlerType(out) != "command" {
		return out
	}
	head, tail, _ := strings.Cut(command, " ")
	rel := strings.TrimPrefix(head, "./")
	if !strings.HasPrefix(rel, "scripts/") {
		return out
	}
	rewritten := target.ScriptCommand(rel)
	if tail != "" {
		rewritten += " " + tail
	}
	out["command"] = rewritten
	return out
}

// Encode renders an owned config as compact JSON for storage in mount state.
func Encode(cfg Config) string {
	content, _ := json.Marshal(cfg)
	return string(content)
}

// Decode parses what Encode produced.
func Decode(content string) (Config, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Hooks == nil {
		cfg.Hooks = map[string][]Group{}
	}
	return cfg, nil
}

// Count returns the number of owned matcher groups.
func Count(cfg Config) int {
	total := 0
	for _, groups := range cfg.Hooks {
		total += len(groups)
	}
	return total
}

// Merge writes owned groups into the file at path, creating it when absent.
// An identical group already present is replaced in place rather than
// duplicated; every other key and group is preserved. created reports
// whether the file did not exist before.
func Merge(path string, owned Config) (created bool, err error) {
	doc, exists, err := readDoc(path)
	if err != nil {
		return false, err
	}
	hooksObj := hooksObject(doc)
	for _, event := range sortedEvents(owned.Hooks) {
		existing := groupsOf(hooksObj, event)
		kept := removeOwned(existing, owned.Hooks[event])
		for _, group := range owned.Hooks[event] {
			kept = append(kept, toAny(group))
		}
		hooksObj[event] = kept
	}
	doc["hooks"] = hooksObj
	return !exists, writeDoc(path, doc)
}

// Remove deletes the owned groups from the file at path and drops empty
// events and the empty hooks object. When nothing else remains, the file
// itself is deleted. deleted reports whether the file was removed.
func Remove(path string, owned Config) (deleted bool, err error) {
	doc, exists, err := readDoc(path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	hooksObj := hooksObject(doc)
	for event, groups := range owned.Hooks {
		kept := removeOwned(groupsOf(hooksObj, event), groups)
		if len(kept) == 0 {
			delete(hooksObj, event)
		} else {
			hooksObj[event] = kept
		}
	}
	if len(hooksObj) == 0 {
		delete(doc, "hooks")
	} else {
		doc["hooks"] = hooksObj
	}
	if len(doc) == 0 {
		return true, os.Remove(path)
	}
	return false, writeDoc(path, doc)
}

// Inspect reports how many owned groups are present in and missing from the
// file at path. A missing file counts every group as missing.
func Inspect(path string, owned Config) (present int, missing int, err error) {
	doc, exists, err := readDoc(path)
	if err != nil {
		return 0, 0, err
	}
	if !exists {
		return 0, Count(owned), nil
	}
	hooksObj := hooksObject(doc)
	for event, groups := range owned.Hooks {
		existing := groupsOf(hooksObj, event)
		for _, group := range groups {
			if containsGroup(existing, group) {
				present++
			} else {
				missing++
			}
		}
	}
	return present, missing, nil
}

func readDoc(path string) (map[string]any, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, false, nil
		}
		return nil, false, err
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return map[string]any{}, true, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, true, fmt.Errorf("%s: %w (csaw only edits well-formed JSON)", path, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, true, nil
}

func writeDoc(path string, doc map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	tmp := path + ".csaw-tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func hooksObject(doc map[string]any) map[string]any {
	if obj, ok := doc["hooks"].(map[string]any); ok {
		return obj
	}
	return map[string]any{}
}

func groupsOf(hooksObj map[string]any, event string) []any {
	if groups, ok := hooksObj[event].([]any); ok {
		return groups
	}
	return nil
}

func removeOwned(existing []any, owned []Group) []any {
	var kept []any
	for _, candidate := range existing {
		if !ownedContains(owned, candidate) {
			kept = append(kept, candidate)
		}
	}
	return kept
}

func ownedContains(owned []Group, candidate any) bool {
	key := canonical(candidate)
	for _, group := range owned {
		if canonical(toAny(group)) == key {
			return true
		}
	}
	return false
}

func containsGroup(existing []any, group Group) bool {
	key := canonical(toAny(group))
	for _, candidate := range existing {
		if canonical(candidate) == key {
			return true
		}
	}
	return false
}

// toAny converts a Group into the generic form a decoded file uses, so the
// two compare canonically.
func toAny(group Group) any {
	content, _ := json.Marshal(group)
	var out any
	_ = json.Unmarshal(content, &out)
	return out
}

// canonical renders a value as JSON with sorted keys (encoding/json sorts
// map keys), giving a stable identity for a matcher group.
func canonical(value any) string {
	content, _ := json.Marshal(value)
	return string(content)
}

func sortedEvents(hooks map[string][]Group) []string {
	events := make([]string, 0, len(hooks))
	for event := range hooks {
		events = append(events, event)
	}
	sort.Strings(events)
	return events
}
