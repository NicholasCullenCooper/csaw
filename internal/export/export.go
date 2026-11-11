// Package export turns a csaw source (or one of its profiles) into a plugin
// that the supported tools' own distribution channels understand: a Claude
// Code plugin (with a marketplace to list several), or an Agent Plugins 1.0
// directory for Codex and ChatGPT. csaw keeps composing and auditing; the
// vendors' channels do the installing.
package export

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/NicholasCullenCooper/csaw/internal/hooks"
	"github.com/NicholasCullenCooper/csaw/internal/mcpmerge"
	"github.com/NicholasCullenCooper/csaw/internal/mount"
	"github.com/NicholasCullenCooper/csaw/internal/skills"
	"github.com/NicholasCullenCooper/csaw/internal/workspace"
)

type Format string

const (
	FormatClaudeCode   Format = "claude-code"
	FormatAgentPlugins Format = "agent-plugins"

	agentPluginsSchema    = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
	agentPluginsMCPSchema = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"
)

// ParseFormat maps user input to a Format.
func ParseFormat(value string) (Format, error) {
	switch Format(value) {
	case FormatClaudeCode, FormatAgentPlugins:
		return Format(value), nil
	case "":
		return FormatClaudeCode, nil
	}
	return "", fmt.Errorf("unknown export format %q (valid: claude-code, agent-plugins)", value)
}

// Input is what a plugin is built from: registry-relative entries of one
// source (optionally narrowed to a profile) and the manifest metadata.
type Input struct {
	Name        string
	Version     string
	Description string
	Entries     []mount.SourceEntry
}

// Result lists what was written and what the format could not carry.
type Result struct {
	Written []string
	Omitted []string
}

var agentPluginName = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

// ValidateName checks a plugin name against the target format's rules.
func ValidateName(format Format, name string) error {
	if name == "" {
		return errors.New("plugin name is empty")
	}
	switch format {
	case FormatAgentPlugins:
		if len(name) > 64 || !agentPluginName.MatchString(name) || strings.Contains(name, "--") || strings.Contains(name, "..") {
			return fmt.Errorf("plugin name %q: Agent Plugins names are 1-64 lowercase letters, digits, hyphens, or periods, start and end alphanumeric, with no -- or ..", name)
		}
	default:
		if strings.ContainsAny(name, " @:/\\\t\n") {
			return fmt.Errorf("plugin name %q: Claude Code names cannot contain spaces, @, :, or path separators", name)
		}
	}
	return nil
}

// Plugin writes a plugin directory at out. out must not exist or must be an
// empty directory unless force is set, in which case its contents are
// replaced.
func Plugin(format Format, in Input, out string, force bool) (Result, error) {
	if err := ValidateName(format, in.Name); err != nil {
		return Result{}, err
	}
	if err := prepareOut(out, force); err != nil {
		return Result{}, err
	}

	var result Result
	write := func(rel string, content []byte, perm os.FileMode) error {
		full := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, content, perm); err != nil {
			return err
		}
		result.Written = append(result.Written, rel)
		return nil
	}
	copyFile := func(rel string, src string) error {
		full := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if _, err := workspace.CopyFile(src, full); err != nil {
			return err
		}
		result.Written = append(result.Written, rel)
		return nil
	}
	omit := func(format string, args ...any) {
		result.Omitted = append(result.Omitted, fmt.Sprintf(format, args...))
	}

	var (
		hooksConfig  *mount.SourceEntry
		mcpJSON      *mount.SourceEntry
		mcpTOML      *mount.SourceEntry
		skippedSkill = map[string]bool{}
		omittedKinds = map[string]int{}
	)
	for i := range in.Entries {
		entry := in.Entries[i]
		rel := entry.RelativePath
		if name, rest, ok := mount.SkillParts(rel); ok {
			if skippedSkill[name] {
				continue
			}
			if rest == "SKILL.md" {
				if err := skills.Validate(name, entry.FullPath); err != nil {
					skippedSkill[name] = true
					omit("skill %s skipped: %v", name, err)
					continue
				}
			}
			if err := copyFile(path.Join("skills", name, rest), entry.FullPath); err != nil {
				return result, err
			}
			continue
		}
		switch kind := mount.KindOf(entry); {
		case rel == "hooks/hooks.json":
			hooksConfig = &in.Entries[i]
		case strings.HasPrefix(rel, "hooks/"):
			if format != FormatClaudeCode {
				omittedKinds["hooks"]++
				continue
			}
			target := strings.TrimPrefix(rel, "hooks/")
			if !strings.HasPrefix(target, "scripts/") {
				target = "scripts/" + target
			}
			if err := copyFile(target, entry.FullPath); err != nil {
				return result, err
			}
		case kind == mount.KindAgent:
			if format != FormatClaudeCode {
				omittedKinds["agents"]++
				continue
			}
			if err := copyFile(path.Join("agents", path.Base(rel)), entry.FullPath); err != nil {
				return result, err
			}
		case rel == "mcp/claude-code.json":
			mcpJSON = &in.Entries[i]
		case rel == "mcp/codex.toml":
			mcpTOML = &in.Entries[i]
		case kind == mount.KindRule:
			omittedKinds["rules"]++
		case kind == mount.KindInstruction:
			omittedKinds["instructions"]++
		case kind == mount.KindIgnore:
			omittedKinds["ignore files"]++
		case kind == mount.KindMCP:
			omittedKinds["other MCP files"]++
		default:
			omittedKinds["other files"]++
		}
	}

	if hooksConfig != nil {
		if format == FormatClaudeCode {
			cfg, err := hooks.Load(hooksConfig.FullPath)
			if err != nil {
				return result, err
			}
			owned, notes := hooks.Plan(cfg, hooks.ClaudePluginTarget)
			result.Omitted = append(result.Omitted, notes...)
			content, err := json.MarshalIndent(map[string]any{"hooks": owned.Hooks}, "", "  ")
			if err != nil {
				return result, err
			}
			if err := write("hooks/hooks.json", append(content, '\n'), 0o644); err != nil {
				return result, err
			}
		} else {
			omittedKinds["hooks"]++
		}
	}

	switch format {
	case FormatClaudeCode:
		if mcpJSON != nil {
			content, err := os.ReadFile(mcpJSON.FullPath)
			if err != nil {
				return result, err
			}
			if err := validateMCPJSON(content); err != nil {
				return result, err
			}
			if err := write(".mcp.json", content, 0o644); err != nil {
				return result, err
			}
		}
		if mcpTOML != nil {
			omit("mcp/codex.toml is Codex-specific and not carried by a Claude Code plugin")
		}
		manifest := map[string]any{"name": in.Name}
		if in.Version != "" {
			manifest["version"] = in.Version
		}
		if in.Description != "" {
			manifest["description"] = in.Description
		}
		if err := write(".claude-plugin/plugin.json", marshal(manifest), 0o644); err != nil {
			return result, err
		}
	case FormatAgentPlugins:
		var servers map[string]any
		var notes []string
		var err error
		switch {
		case mcpTOML != nil:
			servers, notes, err = serversFromTOML(mcpTOML.FullPath)
		case mcpJSON != nil:
			servers, notes, err = serversFromJSON(mcpJSON.FullPath)
		}
		if err != nil {
			return result, err
		}
		result.Omitted = append(result.Omitted, notes...)
		if len(servers) > 0 {
			content := marshal(map[string]any{"$schema": agentPluginsMCPSchema, "mcpServers": servers})
			if err := write("mcp.json", content, 0o644); err != nil {
				return result, err
			}
		}
		manifest := map[string]any{"$schema": agentPluginsSchema, "name": in.Name}
		if in.Version != "" {
			manifest["version"] = in.Version
		}
		if in.Description != "" {
			manifest["description"] = in.Description
		}
		if err := write("plugin.json", marshal(manifest), 0o644); err != nil {
			return result, err
		}
	}

	for _, kind := range sortedKeys(omittedKinds) {
		reason := "not a plugin component; keep mounting them with csaw"
		if format == FormatAgentPlugins && (kind == "agents" || kind == "hooks") {
			reason = "outside the Agent Plugins 1.0 format (skills and MCP only)"
		}
		omit("%d %s omitted: %s", omittedKinds[kind], kind, reason)
	}
	sort.Strings(result.Written)
	return result, nil
}

// MarketplaceEntry is one plugin listed by a Claude Code marketplace.
type MarketplaceEntry struct {
	Name        string
	Source      string // relative path from the marketplace root, e.g. "./plugins/x"
	Description string
}

// Marketplace writes .claude-plugin/marketplace.json at out for entries.
func Marketplace(name string, owner string, entries []MarketplaceEntry, out string) error {
	if name == "" || strings.ContainsAny(name, " \t\n") {
		return fmt.Errorf("marketplace name %q must be non-empty and contain no spaces", name)
	}
	for _, r := range name {
		if r > 127 {
			return fmt.Errorf("marketplace name %q must be ASCII", name)
		}
	}
	if owner == "" {
		owner = "csaw"
	}
	plugins := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(entry.Source, "..") {
			return fmt.Errorf("marketplace source %q must not contain ..", entry.Source)
		}
		item := map[string]any{"name": entry.Name, "source": entry.Source}
		if entry.Description != "" {
			item["description"] = entry.Description
		}
		plugins = append(plugins, item)
	}
	doc := map[string]any{
		"name":    name,
		"owner":   map[string]any{"name": owner},
		"plugins": plugins,
	}
	full := filepath.Join(out, ".claude-plugin", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, marshal(doc), 0o644)
}

func prepareOut(out string, force bool) error {
	entries, err := os.ReadDir(out)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.MkdirAll(out, 0o755)
		}
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if !force {
		return fmt.Errorf("%s is not empty; pass --force to replace its contents", out)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(out, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func marshal(doc map[string]any) []byte {
	content, _ := json.MarshalIndent(doc, "", "  ")
	return append(content, '\n')
}

func validateMCPJSON(content []byte) error {
	var doc struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(content, &doc); err != nil {
		return fmt.Errorf("mcp/claude-code.json: %w", err)
	}
	for name, body := range doc.Servers {
		if err := mcpmerge.ValidateNoLiteralSecrets(name, body); err != nil {
			return fmt.Errorf("mcp/claude-code.json: %w", err)
		}
	}
	return nil
}

// serversFromJSON normalizes a Claude Code .mcp.json into Agent Plugins
// server entries.
func serversFromJSON(path string) (map[string]any, []string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var doc struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, nil, fmt.Errorf("mcp/claude-code.json: %w", err)
	}
	return normalizeServers(doc.Servers, "mcp/claude-code.json")
}

// serversFromTOML normalizes a Codex config fragment ([mcp_servers.*]) into
// Agent Plugins server entries.
func serversFromTOML(path string) (map[string]any, []string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var doc struct {
		Servers map[string]map[string]any `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal(content, &doc); err != nil {
		return nil, nil, fmt.Errorf("mcp/codex.toml: %w", err)
	}
	return normalizeServers(doc.Servers, "mcp/codex.toml")
}

func normalizeServers(servers map[string]map[string]any, origin string) (map[string]any, []string, error) {
	out := map[string]any{}
	var notes []string
	for _, name := range sortedServerNames(servers) {
		body := servers[name]
		if err := mcpmerge.ValidateNoLiteralSecrets(name, body); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", origin, err)
		}
		entry := map[string]any{}
		if url, ok := body["url"].(string); ok && url != "" {
			entry["type"] = "streamable-http"
			if kind, ok := body["type"].(string); ok && kind == "sse" {
				entry["type"] = "sse"
			}
			entry["url"] = url
			for _, key := range []string{"headers", "http_headers"} {
				if headers, ok := body[key].(map[string]any); ok && len(headers) > 0 {
					entry["headers"] = headers
				}
			}
		} else {
			command, ok := body["command"].(string)
			if !ok || command == "" {
				notes = append(notes, fmt.Sprintf("%s: server %s has neither url nor command; skipped", origin, name))
				continue
			}
			entry["type"] = "stdio"
			entry["command"] = command
			if args, ok := body["args"].([]any); ok && len(args) > 0 {
				entry["args"] = args
			}
			if env, ok := body["env"].(map[string]any); ok && len(env) > 0 {
				entry["env"] = env
			}
			if cwd, ok := body["cwd"].(string); ok && cwd != "" {
				entry["cwd"] = cwd
			}
		}
		if names, ok := body["env_vars"].([]any); ok && len(names) > 0 {
			notes = append(notes, fmt.Sprintf("%s: server %s forwards %d env var(s) via env_vars, which Agent Plugins cannot express; set them in the client instead", origin, name, len(names)))
		}
		out[name] = entry
	}
	return out, notes, nil
}

func sortedServerNames(servers map[string]map[string]any) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
