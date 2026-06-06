# Merged-Config MCP Projection: Design Notes

Pre-staged research for a possible future csaw feature: projecting MCP server configurations into target files that csaw does not fully own (Codex `config.toml`, OpenCode `opencode.json`, GitHub Copilot CLI `mcp-config.json`, VS Code workspace `settings.json`). Captured 2026-05-26.

**This document does not commit csaw to building merged-config projection.** Per the working norm captured in [`next-up.md`](next-up.md) ("user-driven over parity-driven"), the actual feature waits for a real user to report friction. The doc exists so when that happens, the answer is one designed swing rather than re-deriving the landscape under pressure.

## What problem this would solve

csaw's MCP projection today is symlink-based: registry files like `mcp/claude-code.json` become symlinks at known project paths (`.mcp.json`, `.cursor/mcp.json`, `.vscode/mcp.json`). This works when the tool reads MCP from a dedicated file. It does not work when the tool reads MCP from a section of a larger config file that also holds user-owned content — auth tokens, model preferences, sandbox config, OAuth state, schedules, etc.

The four tools that motivate the design:

| Tool | MCP file | Format | What else lives there | Scope option(s) |
|---|---|---|---|---|
| Codex CLI | `[mcp_servers.<name>]` tables | TOML | model providers, sandbox, hooks, defaults | `~/.codex/config.toml` (user) **or** `.codex/config.toml` (project) |
| OpenCode | `mcp` object | JSON/JSONC | models, agents, tool config, theme | `~/.config/opencode/opencode.json` (user) **or** `opencode.json` (project) |
| Copilot CLI | `mcpServers` object | JSON | tied to auth state in nearby files | `~/.copilot/mcp-config.json` (user only — no project alternative documented) |
| VS Code workspace | `mcp.servers` or `chat.mcp.*` keys | JSON | massive grab-bag of editor settings | `.vscode/settings.json` (project) |

(Note: VS Code's newer dedicated `.vscode/mcp.json` is already supported by csaw's existing symlink projection; this section is for the older settings-embedded variant if there's user demand.)

The key architectural observation: **three of four targets have project-scope file options**. csaw's project-scope identity stays intact for Codex, OpenCode, and VS Code. Only Copilot CLI requires reaching into user-scope (`~/.copilot/`) — a narrower exception than the original concern suggested.

## Why this is hard

Even confined to project-scope files, the merge problem has five hard sub-problems:

### 1. Round-trip formatting preservation

If csaw parses a file with library X, modifies it, and writes it back, library X almost certainly drops comments, reorders keys, and reformats whitespace. Bad round-trip turns a user's hand-formatted config into machine-formatted noise — users will hate csaw.

**Go libraries that preserve formatting reasonably well:**

- **TOML:** [`pelletier/go-toml/v2`](https://github.com/pelletier/go-toml) — has marshal/unmarshal with formatting; comment preservation is limited but better than `BurntSushi/toml`. Real audit needed before committing.
- **JSON (surgical edits):** [`tidwall/sjson`](https://github.com/tidwall/sjson) is the right primitive. It sets values at JSON paths without reformatting the surrounding document. Crucially, it preserves whitespace and key order outside the touched paths. This avoids the parse-modify-serialize roundtrip entirely.
- **JSONC:** [`tailscale/hujson`](https://github.com/tailscale/hujson) parses JSON with comments and trailing commas; preserves comments on round-trip. Can be combined with sjson-style path edits if we write a small helper.

The cleanest implementation pattern: use sjson/hujson-style **surgical edits** rather than full parse-modify-serialize. Set the exact paths csaw owns; touch nothing else.

### 2. State tracking for unmount

With symlinks, `csaw unmount` is `os.Remove(symlink)` — atomic, complete. With merged files, unmount means "remove csaw's entries; leave everything else byte-identical (or as close as possible)."

**Required state:** a per-target-file manifest stored under `~/.csaw/state/`:

```json
{
  "target_file": "/path/to/project/.codex/config.toml",
  "owned_paths": [
    {"path": "mcp_servers.github", "sha": "..."},
    {"path": "mcp_servers.linear", "sha": "..."}
  ],
  "written_at": "2026-05-26T..."
}
```

On unmount: read manifest → for each owned path, check that current value SHA matches what csaw wrote → if match, delete the entry → if drift, refuse to remove and surface "user edited this entry; you own it now."

### 3. Conflict resolution

When a source's MCP config and the user's existing config both define an entry with the same name, what happens?

**Recommended default: user-wins, csaw skips with verbose report.**

```
csaw use team/backend
  ✔ mounted .claude/rules/security.md from team-source
  ⚠ skipped MCP server "github" in .codex/config.toml — already defined by user
    Use `csaw mcp diff codex` to see the difference.
```

Source files can opt-in to `must_apply: true` for entries that override user config — this inverts today's protected-files model (where source protects FROM override; here source declares "this MUST be deployed even if user has their own"). Should be rare and clearly visible at mount time.

### 4. Secrets

Source registries are git-committed and often shared across team. Literal API keys in source files would be a critical security failure. Two complementary defenses:

**(a) Schema enforcement — refuse to project literal secrets.**

The registry MCP fragment file (`mcp/codex.toml`, `mcp/opencode.json`, etc.) must use a portable env-reference syntax. csaw validates at mount time and refuses if any sensitive-named field contains a literal value.

Proposed syntax: `${env:VAR_NAME}` — csaw translates to each tool's native form on write:

| Tool | Native form |
|---|---|
| Codex | `env_vars = ["VAR_NAME"]` (variable forwarded by Codex; value never appears in config) |
| OpenCode | Native env var substitution syntax (TBD; verify at build time) |
| Copilot CLI | Standard JSON; values must be `${VAR}` references |
| VS Code | VS Code's `${env:VAR}` syntax (already a thing in `settings.json`) |

**Sensitive-named fields** that trigger validation: `token`, `secret`, `password`, `api_key`, `apiKey`, `private_key`, `privateKey`, `bearer_token`, anything ending in `_token`/`_secret`/`_key` (excluding `public_key`).

**(b) Entropy/pattern detection as a second-layer warning.**

Even non-sensitive-named fields can contain secrets accidentally. Heuristics borrowed from gitleaks/trufflehog: warn on strings matching common token prefixes (`ghp_`, `sk-`, `gho_`, `AKIA`, `eyJ` JWT prefix) or high-entropy alphanumeric strings >20 chars. Warn, don't refuse — false positives are user-hostile when blocking, acceptable when alerting.

### 5. Audit

`csaw audit` today checks mounted state (symlink targets, protected-file hashes). Merged-state audit requires re-parsing the target file at audit time and comparing owned-paths manifest against current file contents. New finding types:

- `MCP_SERVER_REMOVED` — csaw owns this entry but it's no longer in the file
- `MCP_SERVER_DRIFTED` — csaw wrote SHA X; file now has SHA Y
- `MCP_SERVER_CONFLICT` — user has same-named entry as source (skipped at mount)
- `MCP_SECRET_LEAK` — sensitive-named field contains literal value (validation failure)

## Recommended scope cuts when we build this

If we ever decide to build merged-config MCP projection, start narrow and validate before widening:

### Scope cut 1: One tool first (Codex)

Codex's `.codex/config.toml` is project-scope and uses TOML — the trickiest format for round-trip. If we can make Codex work cleanly, JSON-based tools (OpenCode, Copilot CLI, VS Code) are easier. If TOML round-trip turns out to be a tar pit, we learn fast and either ship Option-1-style docs only or invest in better TOML tooling.

### Scope cut 2: Separate command from `csaw mount`

Don't fold merge into `csaw mount`. Use a dedicated `csaw mcp sync <tool>` (or similar) command. Why:

- Keeps `csaw mount`'s "I am a symlink projector" identity intact. Reversible by design is a v1.0 promise.
- Makes the merge step explicit. User opts in per command invocation.
- Per-tool scoping matches the per-tool nature of the merge problem.
- Different rollback semantics: `csaw unmount` doesn't touch merged files; `csaw mcp sync <tool> --remove` does.

### Scope cut 3: Project-scope only (defer Copilot CLI)

Build for the three project-scope targets first. Don't touch user-scope (`~/.copilot/`) until project-scope works. When/if we add user-scope:

- Require explicit `--user-scope` flag (no implicit user-scope writes)
- User-scope state lives in a separate manifest (`~/.csaw/state/user-scope-merges/`)
- User-scope unmount is a separate command (`csaw mcp user-scope unmount <tool>`) — never automatic
- All user-scope writes logged to a forensic-recovery log in case of disaster

### Scope cut 4: Dry-run first, apply second

`csaw mcp sync <tool>` defaults to dry-run. Show the diff. Require `--apply` to actually write. This is the rsync/terraform pattern; well-understood.

## Source registry format

Today source registries have `mcp/<tool>.json` files that are whole files (e.g., `mcp/claude-code.json` is the entire `.mcp.json` content). For merged-config tools, the source registry needs **fragments** — just the MCP entries, not the surrounding file:

**Codex fragment example** (`mcp/codex.toml`):
```toml
[mcp_servers.github]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
env_vars = ["GITHUB_PERSONAL_ACCESS_TOKEN"]

[mcp_servers.linear]
command = "npx"
args = ["-y", "@tacticiq/linear-mcp"]
env_vars = ["LINEAR_API_KEY"]
```

**OpenCode fragment example** (`mcp/opencode.json` or `.jsonc`):
```jsonc
{
  // csaw merges these entries under the top-level mcp object
  "mcp": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "${env:WORKSPACE_DIR}"],
      "type": "stdio"
    }
  }
}
```

Naming distinction: `mcp/claude-code.json` (existing, whole-file symlink) vs `mcp/codex.toml` (proposed, fragment merged). csaw's kind classifier can distinguish by registry-target filename pattern.

## Per-profile opt-in

Per the user-driven principle, merge behavior must be opt-in at the profile level:

```yaml
# csaw.yml
backend:
  description: Backend dev with MCP for Codex
  include:
    - rules/**
    - agents/**
  mcp_merge:
    - codex          # merge mcp/codex.toml into .codex/config.toml at mount time
    - opencode       # merge mcp/opencode.jsonc into opencode.json
```

Default: no merge. Active opt-in required per profile per tool. No magic, no hidden defaults.

## Open design questions

These are the things that need answering when we actually build. Capturing now so they're not re-derived:

1. **Should the source fragment format be the same as the tool's native format, or a portable canonical form?** TOML for Codex, JSONC for OpenCode is annoying (two formats to maintain). A canonical YAML or JSON5 fragment that csaw transpiles to each tool's native form is cleaner but more code. Probably native-form-per-tool is the right v1; canonical comes later.

2. **What's the conflict report verbosity at mount time?** A summary line + pointer to `csaw mcp diff`? Or inline diff in `csaw use` output? Probably summary + pointer; mount output already has a lot.

3. **Does `csaw audit` re-parse target files on every run?** That's a real perf cost for big config files. Should the manifest cache the file SHA so audit can short-circuit when unchanged?

4. **How does `csaw mcp sync` interact with `csaw use`?** Does `csaw use` invoke the merge when a profile declares `mcp_merge:`, or is it always a separate command the user runs? Probably: `csaw use` performs the merge as part of mount when opted in (matches `mcp_merge:` config); `csaw mcp sync` is the manual escape hatch.

5. **What happens on `csaw unmount` when merged entries exist?** Auto-remove (mirrors symlink behavior) or leave in place (different rollback semantics)? Probably auto-remove with the drift-detection logic from §2 above — but a `--keep-mcp` flag for users who want to leave the entries in place.

6. **VS Code's `.vscode/settings.json` — is the merge actually wanted?** The newer `.vscode/mcp.json` (already symlink-projected by csaw) is the recommended path. settings.json merging would only matter if a real user reports needing it. Currently lowest priority of the four; could be dropped from scope.

## What this design lets us decline

Pre-stating the things we're NOT planning to do, so they don't creep in:

- Not a config-management layer for non-MCP keys. csaw doesn't merge model preferences, sandbox config, hooks, etc. — only MCP entries.
- Not user-scope by default. Project-scope first; user-scope is an explicit later expansion if demanded.
- Not a secret manager. csaw refuses to project literal secrets but doesn't store, encrypt, or transmit them.
- Not a runtime hook. csaw writes config and leaves; the tool reads config on its own startup.

## Path forward

1. **Stay symlink-only today.** Update [`projection-roadmap.md`](projection-roadmap.md) with a "merged-config tools" section pointing readers here.
2. **Wait for a user report.** A real csaw user saying "I want my team's MCP entries in Codex without manually copying" is the trigger.
3. **When triggered, build Scope Cut 1+2+3+4 above as a prototype** (one tool, separate command, project-scope, dry-run first). 2–3 week investment.
4. **Re-evaluate after prototype.** If round-trip + state tracking + secret detection all hold up cleanly, widen to other tools. If any of them turn into tar pits, ship the docs and stop honestly.

Sources used to ground this design (verify currency at build time):

- [Codex Configuration Reference](https://developers.openai.com/codex/config-reference) (OpenAI Developers)
- [Codex MCP](https://developers.openai.com/codex/mcp)
- [OpenCode Config](https://opencode.ai/docs/config/)
- [OpenCode MCP servers](https://opencode.ai/docs/mcp-servers/)
- [Composio: MCP with Codex](https://composio.dev/content/how-to-mcp-with-codex)
- [Composio: MCP with OpenCode](https://composio.dev/content/mcp-with-opencode)
